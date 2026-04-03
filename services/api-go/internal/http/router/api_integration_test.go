package router

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"concordance/services/api-go/internal/config"
	"concordance/services/api-go/internal/db"
	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/http/handlers"
	"concordance/services/api-go/internal/projects"
)

type projectResponse struct {
	ID string `json:"id"`
}

type documentResponse struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

type passageResponse struct {
	ID string `json:"id"`
}

type sentenceResponse struct {
	ID string `json:"id"`
}

type pipelineJobResponse struct {
	Stage  string `json:"stage"`
	Status string `json:"status"`
}

func TestProjectUploadStatusFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from projects"); err != nil {
		t.Fatalf("reset projects table: %v", err)
	}

	uploadDir := t.TempDir()
	health := handlers.NewHealthHandler(config.Config{}, handlers.ReadinessChecks{Database: pool.Ping})
	projectHandler := handlers.NewProjectsHandler(projects.NewPostgresStore(pool))
	documentsHandler := handlers.NewDocumentsHandler(documents.NewPostgresStore(pool), uploadDir)

	testServer := httptest.NewServer(New(health, projectHandler, documentsHandler))
	defer testServer.Close()

	projectID := createProject(t, testServer.URL)
	docID := uploadDocument(t, testServer.URL, projectID)

	status := waitForDocumentReady(t, testServer.URL, docID)
	if status.Status != "ready" {
		t.Fatalf("unexpected final document status: got=%s", status.Status)
	}

	if status.Progress != 1 {
		t.Fatalf("unexpected final document progress: got=%v", status.Progress)
	}

	passages := fetchPassages(t, testServer.URL, docID)
	if len(passages) == 0 {
		t.Fatalf("expected persisted passages")
	}

	sentences := fetchSentences(t, testServer.URL, docID)
	if len(sentences) == 0 {
		t.Fatalf("expected persisted sentences")
	}

	jobs := fetchPipelineJobs(t, testServer.URL, docID)
	if len(jobs) == 0 {
		t.Fatalf("expected persisted pipeline jobs")
	}

	matches, err := filepath.Glob(filepath.Join(uploadDir, "*.txt"))
	if err != nil {
		t.Fatalf("glob uploaded files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one uploaded file, got=%d", len(matches))
	}
}

func createProject(t *testing.T, baseURL string) string {
	t.Helper()

	body := []byte(`{"name":"Integration Suite"}`)
	resp, err := http.Post(baseURL+"/api/projects", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create project request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status: got=%d", resp.StatusCode)
	}

	var payload projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode create project response: %v", err)
	}

	if payload.ID == "" {
		t.Fatalf("expected created project id")
	}

	return payload.ID
}

func fetchPassages(t *testing.T, baseURL, documentID string) []passageResponse {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/passages")
	if err != nil {
		t.Fatalf("passages request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("passages status code: got=%d", resp.StatusCode)
	}

	var payload listResponse[passageResponse]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode passages response: %v", err)
	}

	return payload.Items
}

func fetchSentences(t *testing.T, baseURL, documentID string) []sentenceResponse {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/sentences")
	if err != nil {
		t.Fatalf("sentences request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sentences status code: got=%d", resp.StatusCode)
	}

	var payload listResponse[sentenceResponse]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode sentences response: %v", err)
	}

	return payload.Items
}

func fetchPipelineJobs(t *testing.T, baseURL, documentID string) []pipelineJobResponse {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/pipeline-jobs")
	if err != nil {
		t.Fatalf("pipeline jobs request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pipeline jobs status code: got=%d", resp.StatusCode)
	}

	var payload listResponse[pipelineJobResponse]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode pipeline jobs response: %v", err)
	}

	return payload.Items
}

func uploadDocument(t *testing.T, baseURL, projectID string) string {
	t.Helper()

	form := new(bytes.Buffer)
	writer := multipart.NewWriter(form)
	part, err := writer.CreateFormFile("file", "sample.txt")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	_, _ = part.Write([]byte("Call me Ishmael."))
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

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("upload status: got=%d", resp.StatusCode)
	}

	var payload documentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	if payload.ID == "" {
		t.Fatalf("expected created document id")
	}

	return payload.ID
}

func waitForDocumentReady(t *testing.T, baseURL, documentID string) documentResponse {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/documents/" + documentID + "/status")
		if err != nil {
			t.Fatalf("status request failed: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("status request code: got=%d", resp.StatusCode)
		}

		var payload documentResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			t.Fatalf("decode status response: %v", err)
		}
		resp.Body.Close()

		if payload.Status == "ready" {
			return payload
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("document did not reach ready state before timeout")
	return documentResponse{}
}
