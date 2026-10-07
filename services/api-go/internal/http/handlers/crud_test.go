package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/projects"
)

func TestProjectRenameAndDelete(t *testing.T) {
	store := projects.NewMemoryStore()
	h := NewProjectsHandler(store)
	project, _ := store.Create(context.Background(), projects.CreateProjectInput{Name: "Old"})

	req := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"name":"New","description":"d"}`))
	req.SetPathValue("projectId", project.ID)
	rr := httptest.NewRecorder()
	h.UpdateProject(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rename: got=%d", rr.Code)
	}

	blank := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"name":"  "}`))
	blank.SetPathValue("projectId", project.ID)
	rr = httptest.NewRecorder()
	h.UpdateProject(rr, blank)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("blank rename: got=%d", rr.Code)
	}

	missing := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"name":"x"}`))
	missing.SetPathValue("projectId", "nope")
	rr = httptest.NewRecorder()
	h.UpdateProject(rr, missing)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing rename: got=%d", rr.Code)
	}

	del := httptest.NewRequest(http.MethodDelete, "/", nil)
	del.SetPathValue("projectId", project.ID)
	rr = httptest.NewRecorder()
	h.DeleteProject(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: got=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.DeleteProject(rr, del)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second delete: got=%d", rr.Code)
	}
}

func TestDeleteProjectRemovesUploadedFiles(t *testing.T) {
	events := documents.NewJobEventBroker()
	docs := documents.NewMemoryStore(events)
	store := projects.NewMemoryStore()
	h := NewProjectsHandler(store).WithDocuments(docs)

	project, _ := store.Create(context.Background(), projects.CreateProjectInput{Name: "P"})
	file := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = docs.Create(context.Background(), documents.CreateInput{ProjectID: project.ID, FileName: "book.txt", LocalPath: file})

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("projectId", project.ID)
	rr := httptest.NewRecorder()
	h.DeleteProject(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: got=%d", rr.Code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("uploaded file should be gone, stat err=%v", err)
	}
}

func TestDocumentRenameAndDelete(t *testing.T) {
	events := documents.NewJobEventBroker()
	store := documents.NewMemoryStore(events)
	h := NewDocumentsHandler(store, t.TempDir(), events)
	doc, _ := store.Create(context.Background(), documents.CreateInput{ProjectID: "p", FileName: "a.txt"})

	req := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"fileName":"b.txt"}`))
	req.SetPathValue("documentId", doc.ID)
	rr := httptest.NewRecorder()
	h.RenameDocument(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rename: got=%d", rr.Code)
	}
	if got, _ := store.Get(context.Background(), doc.ID); got.FileName != "b.txt" {
		t.Fatalf("name not updated: %q", got.FileName)
	}

	blank := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"fileName":""}`))
	blank.SetPathValue("documentId", doc.ID)
	rr = httptest.NewRecorder()
	h.RenameDocument(rr, blank)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("blank rename: got=%d", rr.Code)
	}

	del := httptest.NewRequest(http.MethodDelete, "/", nil)
	del.SetPathValue("documentId", doc.ID)
	rr = httptest.NewRecorder()
	h.DeleteDocument(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: got=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.DeleteDocument(rr, del)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second delete: got=%d", rr.Code)
	}
}
