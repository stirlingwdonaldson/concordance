package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"concordance/services/api-go/internal/documents"
)

type kwicListPayload struct {
	Items  []documents.KWICOccurrence `json:"items"`
	Total  int                        `json:"total"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}

func TestUploadDocument(t *testing.T) {
	dir := t.TempDir()
	events := documents.NewJobEventBroker()
	store := documents.NewMemoryStore(events)
	h := NewDocumentsHandler(store, dir, events)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "sample.txt")
	if err != nil {
		t.Fatalf("unable to create form file: %v", err)
	}
	_, _ = part.Write([]byte("hello world"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("projectId", "p1")
	rr := httptest.NewRecorder()

	h.UploadDocument(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var doc documents.Document
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("unable to parse payload: %v", err)
	}

	if !strings.HasPrefix(doc.LocalPath, dir) {
		t.Fatalf("expected local path under upload dir: got=%s", doc.LocalPath)
	}

	if filepath.Ext(doc.LocalPath) != ".txt" {
		t.Fatalf("expected txt extension in local path: got=%s", doc.LocalPath)
	}

	if _, err := os.Stat(doc.LocalPath); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	loaded, ok := store.Get(context.Background(), doc.ID)
	if !ok {
		t.Fatalf("expected uploaded document to be available")
	}
	if loaded.Status == "queued" {
		t.Fatalf("expected pipeline to begin processing")
	}
}

func TestGetDocumentStatusNotFound(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/missing/status", nil)
	req.SetPathValue("documentId", "missing")
	rr := httptest.NewRecorder()

	h.GetDocumentStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListPassages(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/passages", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListPassages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListSentences(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/sentences", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListSentences(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListPipelineJobs(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/pipeline-jobs", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListPipelineJobs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListConcordance(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/concordance?lemma=ishmael&pos=X", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListConcordance(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListKWIC(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/kwic?lemma=ishmael&limit=10&offset=5", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListKWIC(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var payload kwicListPayload
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.Total != 0 {
		t.Fatalf("expected total=0, got=%d", payload.Total)
	}
	if payload.Limit != 10 {
		t.Fatalf("expected limit=10, got=%d", payload.Limit)
	}
	if payload.Offset != 5 {
		t.Fatalf("expected offset=5, got=%d", payload.Offset)
	}
}

func TestListKWICClampsInvalidPaging(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodGet, "/api/documents/d1/kwic?limit=-1&offset=-2", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.ListKWIC(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var payload kwicListPayload
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.Limit != 50 {
		t.Fatalf("expected clamped limit=50, got=%d", payload.Limit)
	}
	if payload.Offset != 0 {
		t.Fatalf("expected clamped offset=0, got=%d", payload.Offset)
	}
}

func TestRetryDocumentNotFound(t *testing.T) {
	events := documents.NewJobEventBroker()
	h := NewDocumentsHandler(documents.NewMemoryStore(events), t.TempDir(), events)
	req := httptest.NewRequest(http.MethodPost, "/api/documents/d1/retry", nil)
	req.SetPathValue("documentId", "d1")
	rr := httptest.NewRecorder()

	h.RetryDocument(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestRetryDocumentAccepted(t *testing.T) {
	dir := t.TempDir()
	events := documents.NewJobEventBroker()
	store := documents.NewMemoryStore(events)
	doc, err := store.Create(context.Background(), documents.CreateInput{
		ProjectID:  "p1",
		FileName:   "sample.txt",
		LocalPath:  filepath.Join(dir, "sample.txt"),
		SourceHash: "h1",
		Format:     "txt",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}
	h := NewDocumentsHandler(store, dir, events)
	req := httptest.NewRequest(http.MethodPost, "/api/documents/"+doc.ID+"/retry", nil)
	req.SetPathValue("documentId", doc.ID)
	rr := httptest.NewRecorder()

	h.RetryDocument(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}
