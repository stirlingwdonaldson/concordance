package handlers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"concordance/services/api-go/internal/documents"
)

type DocumentsHandler struct {
	store     documents.Store
	uploadDir string
}

func NewDocumentsHandler(store documents.Store, uploadDir string) DocumentsHandler {
	return DocumentsHandler{store: store, uploadDir: uploadDir}
}

func (h DocumentsHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	if strings.TrimSpace(projectID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing project id"})
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart payload"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file field"})
		return
	}
	defer file.Close()

	localPath, err := saveUploadFile(h.uploadDir, header.Filename, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to save upload"})
		return
	}

	doc, err := h.store.Create(r.Context(), projectID, header.Filename, localPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to register document"})
		return
	}

	go h.store.RunPipeline(context.Background(), doc.ID)

	writeJSON(w, http.StatusAccepted, doc)
}

func (h DocumentsHandler) GetDocumentStatus(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	doc, ok := h.store.Get(r.Context(), documentID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (h DocumentsHandler) ListProjectDocuments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	items, err := h.store.ListByProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list project documents"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func saveUploadFile(uploadDir, fileName string, reader io.Reader) (string, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	targetPath := filepath.Join(uploadDir, filepath.Base(fileName))
	output, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer output.Close()

	if _, err := io.Copy(output, reader); err != nil {
		return "", err
	}

	return targetPath, nil
}
