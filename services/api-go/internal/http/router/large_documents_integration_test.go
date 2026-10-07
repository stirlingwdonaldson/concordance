package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"concordance/services/api-go/internal/config"
	"concordance/services/api-go/internal/db"
	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/http/handlers"
	"concordance/services/api-go/internal/nlpv1"
	"concordance/services/api-go/internal/projects"
)

// requestCeilingBytes stands in for the gRPC message limit: the stub refuses any
// single request carrying more text than this, the way the real sidecar
// connection used to refuse oversized messages.
const requestCeilingBytes = 450_000

// recordingNLPClient tokenizes on whitespace and records how it was called.
type recordingNLPClient struct {
	mu           sync.Mutex
	calls        int
	largestBytes int
	failFirst    bool
	failed       atomic.Bool
}

func (c *recordingNLPClient) Health(_ context.Context) (*nlpv1.HealthResponse, error) {
	return &nlpv1.HealthResponse{Status: "ready"}, nil
}

func (c *recordingNLPClient) AnalyzeDocument(_ context.Context, req *nlpv1.AnalyzeDocumentRequest) (*nlpv1.AnalyzeDocumentResponse, error) {
	if c.failFirst && c.failed.CompareAndSwap(false, true) {
		return nil, errors.New("simulated sidecar failure")
	}

	size := 0
	for _, passage := range req.Passages {
		for _, sentence := range passage.Sentences {
			size += len(sentence.Text)
		}
	}

	c.mu.Lock()
	c.calls++
	if size > c.largestBytes {
		c.largestBytes = size
	}
	c.mu.Unlock()

	if size > requestCeilingBytes {
		return nil, fmt.Errorf("request of %d bytes exceeds ceiling of %d", size, requestCeilingBytes)
	}

	tokens := make([]*nlpv1.Token, 0)
	for _, passage := range req.Passages {
		for _, sentence := range passage.Sentences {
			tokens = append(tokens, wordTokens(sentence)...)
		}
	}

	return &nlpv1.AnalyzeDocumentResponse{Tokens: tokens}, nil
}

func (c *recordingNLPClient) Close() error { return nil }

func wordTokens(sentence *nlpv1.SentenceInput) []*nlpv1.Token {
	text := sentence.Text
	tokens := make([]*nlpv1.Token, 0)

	index, i := 0, 0
	for i < len(text) {
		for i < len(text) && strings.ContainsRune(" \t\n", rune(text[i])) {
			i++
		}
		start := i
		for i < len(text) && !strings.ContainsRune(" \t\n", rune(text[i])) {
			i++
		}
		if start == i {
			break
		}

		surface := text[start:i]
		lemma, pos := strings.ToLower(strings.Trim(surface, ".,;!?")), "X"
		switch lemma {
		case "runs", "ran", "running":
			lemma, pos = "run", "VERB"
		case "cat", "cats", "dog", "dogs":
			lemma, pos = strings.TrimSuffix(lemma, "s"), "NOUN"
		}
		tokens = append(tokens, &nlpv1.Token{
			SentenceId: sentence.SentenceId,
			TokenIndex: int32(index),
			Surface:    surface,
			Lemma:      lemma,
			Pos:        pos,
			StartChar:  sentence.StartChar + int64(start),
			EndChar:    sentence.StartChar + int64(i),
		})
		index++
	}

	return tokens
}

type integrationServer struct {
	url       string
	uploadDir string
}

func newIntegrationServer(t *testing.T, nlpClient *recordingNLPClient) integrationServer {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.RunMigrations(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from projects"); err != nil {
		t.Fatalf("reset projects: %v", err)
	}

	uploadDir := t.TempDir()
	events := documents.NewJobEventBroker()
	health := handlers.NewHealthHandler(config.Config{}, handlers.ReadinessChecks{Database: pool.Ping})
	projectHandler := handlers.NewProjectsHandler(projects.NewPostgresStore(pool))
	documentsHandler := handlers.NewDocumentsHandler(documents.NewPostgresStore(pool, events, nlpClient), uploadDir, events)

	server := httptest.NewServer(New(health, projectHandler, documentsHandler))
	t.Cleanup(server.Close)

	return integrationServer{url: server.URL, uploadDir: uploadDir}
}

func uploadFile(t *testing.T, baseURL, projectID, fileName string, content []byte) (documentResponse, int) {
	t.Helper()

	form := new(bytes.Buffer)
	writer := multipart.NewWriter(form)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	_, _ = part.Write(content)
	_ = writer.Close()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/projects/"+projectID+"/documents/upload", form)
	if err != nil {
		t.Fatalf("build upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload documentResponse
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	return payload, resp.StatusCode
}

// waitForTerminal waits until the document is ready or failed and returns the
// raw status payload so failures can be reported with their reason.
func waitForTerminal(t *testing.T, baseURL, documentID string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/status")
		if err != nil {
			t.Fatalf("status request failed: %v", err)
		}

		var payload map[string]any
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("decode status: %v", err)
		}

		if status, _ := payload["status"].(string); status == "ready" || status == "failed" {
			return payload
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("document %s did not finish within %s", documentID, timeout)
	return nil
}

func syntheticBook(words int) (text string, zetaCount int) {
	vocabulary := strings.Fields("the of and to in a is that for it as was with be by on not he this are or his from at which but have an had they you were their one all we can her has there been if more when will would who so no")

	var b strings.Builder
	sentenceWords, sentences := 0, 0
	for i := 1; i <= words; i++ {
		word := vocabulary[(i*7+i/13)%len(vocabulary)]
		if i%100 == 0 {
			word = "zeta"
			zetaCount++
		}

		if sentenceWords > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(word)
		sentenceWords++

		if sentenceWords == 12 {
			b.WriteByte('.')
			sentences++
			sentenceWords = 0
			if sentences%8 == 0 {
				b.WriteString("\n\n")
			} else {
				b.WriteByte(' ')
			}
		}
	}

	return b.String(), zetaCount
}

func kwicTotal(t *testing.T, baseURL, documentID, lemma string) int {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/kwic?limit=1&lemma=" + lemma)
	if err != nil {
		t.Fatalf("kwic request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("kwic status: got=%d", resp.StatusCode)
	}

	var payload kwicListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode kwic: %v", err)
	}

	return payload.Total
}

func TestLargeDocumentIsAnalyzedInBatches(t *testing.T) {
	nlpClient := &recordingNLPClient{}
	server := newIntegrationServer(t, nlpClient)
	projectID := createProject(t, server.url)

	book, wantZeta := syntheticBook(300_000)
	if len(book) < 2*requestCeilingBytes {
		t.Fatalf("test book is unexpectedly small: %d bytes", len(book))
	}

	doc, code := uploadFile(t, server.url, projectID, "textbook.txt", []byte(book))
	if code != http.StatusAccepted {
		t.Fatalf("upload status: got=%d", code)
	}

	started := time.Now()
	status := waitForTerminal(t, server.url, doc.ID, 3*time.Minute)
	t.Logf("300k-word document processed in %s with %d analysis call(s)", time.Since(started).Round(time.Millisecond), nlpClient.calls)
	if status["status"] != "ready" {
		t.Fatalf("large document did not become ready: %v", status)
	}

	if nlpClient.calls < 2 {
		t.Fatalf("expected the document to be analyzed in several batches, got %d call(s)", nlpClient.calls)
	}
	if nlpClient.largestBytes > requestCeilingBytes {
		t.Fatalf("a request exceeded the ceiling: %d", nlpClient.largestBytes)
	}

	if got := kwicTotal(t, server.url, doc.ID, "zeta"); got != wantZeta {
		t.Fatalf("every occurrence must survive batching: got=%d want=%d", got, wantZeta)
	}
}

func TestDuplicateUploadResolvesInsteadOfFailing(t *testing.T) {
	nlpClient := &recordingNLPClient{failFirst: true}
	server := newIntegrationServer(t, nlpClient)
	projectID := createProject(t, server.url)
	content := []byte("It was the best of times. It was the worst of times.")

	first, code := uploadFile(t, server.url, projectID, "dickens.txt", content)
	if code != http.StatusAccepted {
		t.Fatalf("first upload status: got=%d", code)
	}
	if status := waitForTerminal(t, server.url, first.ID, 30*time.Second); status["status"] != "failed" {
		t.Fatalf("expected the first attempt to fail, got %v", status["status"])
	}

	// Re-uploading the same file after a failure must act as a retry.
	second, code := uploadFile(t, server.url, projectID, "dickens.txt", content)
	if code != http.StatusAccepted {
		t.Fatalf("re-upload status: got=%d (this used to be a 500)", code)
	}
	if second.ID != first.ID {
		t.Fatalf("re-upload should resolve to the existing document: %s vs %s", second.ID, first.ID)
	}
	if status := waitForTerminal(t, server.url, second.ID, 30*time.Second); status["status"] != "ready" {
		t.Fatalf("re-uploaded document did not recover: %v", status)
	}

	// Uploading it a third time must not reprocess or fail.
	callsBefore := nlpClient.calls
	third, code := uploadFile(t, server.url, projectID, "dickens.txt", content)
	if code != http.StatusAccepted || third.ID != first.ID {
		t.Fatalf("duplicate of a ready document: code=%d id=%s", code, third.ID)
	}
	time.Sleep(300 * time.Millisecond)
	if nlpClient.calls != callsBefore {
		t.Fatalf("a ready document must not be reprocessed by a duplicate upload")
	}

	// The same file in a different project is a separate document.
	otherProject := createProject(t, server.url)
	other, code := uploadFile(t, server.url, otherProject, "dickens.txt", content)
	if code != http.StatusAccepted || other.ID == first.ID {
		t.Fatalf("same file in another project: code=%d id=%s", code, other.ID)
	}
	if status := waitForTerminal(t, server.url, other.ID, 30*time.Second); status["status"] != "ready" {
		t.Fatalf("document in second project did not become ready: %v", status)
	}

	matches, _ := filepath.Glob(filepath.Join(server.uploadDir, "*.txt"))
	if len(matches) != 2 {
		t.Fatalf("duplicate uploads should not leave extra copies on disk: got=%d files", len(matches))
	}
}

func TestMessyEncodingsAndControlCharactersStillIngest(t *testing.T) {
	server := newIntegrationServer(t, &recordingNLPClient{})
	projectID := createProject(t, server.url)

	// Windows-1252 bytes, a NUL, and a form feed: each of these used to be able
	// to make the database insert fail.
	messy := []byte("The caf\xe9 was \x93open\x94.\x00 A new page\x0cbegins here.")

	doc, code := uploadFile(t, server.url, projectID, "legacy.txt", messy)
	if code != http.StatusAccepted {
		t.Fatalf("upload status: got=%d", code)
	}

	if status := waitForTerminal(t, server.url, doc.ID, 30*time.Second); status["status"] != "ready" {
		t.Fatalf("messy document did not become ready: %v", status)
	}

	if total := kwicTotal(t, server.url, doc.ID, "new"); total != 1 {
		t.Fatalf("expected the cleaned text to be indexed, kwic total=%d", total)
	}
}
