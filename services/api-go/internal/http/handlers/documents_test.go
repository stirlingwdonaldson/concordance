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
	"testing"
	"time"

	"concordance/services/api-go/internal/documents"
)

func TestUploadDocument(t *testing.T) {
	dir := t.TempDir()
	store := documents.NewMemoryStore()
	h := NewDocumentsHandler(store, dir)

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

	if _, err := os.Stat(filepath.Join(dir, "sample.txt")); err != nil {
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
	h := NewDocumentsHandler(documents.NewMemoryStore(), t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/documents/missing/status", nil)
	req.SetPathValue("documentId", "missing")
	rr := httptest.NewRecorder()

	h.GetDocumentStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}
