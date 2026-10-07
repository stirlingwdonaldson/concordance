package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"concordance/services/api-go/internal/documents"
)

type DocumentsHandler struct {
	store     documents.Store
	uploadDir string
	events    *documents.JobEventBroker
}

func NewDocumentsHandler(store documents.Store, uploadDir string, events *documents.JobEventBroker) DocumentsHandler {
	return DocumentsHandler{store: store, uploadDir: uploadDir, events: events}
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

	localPath, sourceHash, err := saveUploadFile(h.uploadDir, header.Filename, file)
	if err != nil {
		log.Printf("upload: save failed for %q: %v", header.Filename, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to save upload"})
		return
	}

	doc, err := h.store.Create(r.Context(), documents.CreateInput{
		ProjectID:  projectID,
		FileName:   header.Filename,
		LocalPath:  localPath,
		SourceHash: sourceHash,
		Format:     detectFormat(header.Filename),
	})
	if err != nil {
		log.Printf("upload: register failed for %q in project %s: %v", header.Filename, projectID, err)
		_ = os.Remove(localPath)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to register document"})
		return
	}

	// A duplicate upload resolves to the existing document, so the copy we just
	// saved is redundant.
	if doc.LocalPath != localPath {
		_ = os.Remove(localPath)
	}

	// Only start processing for newly queued documents; an already-processed
	// duplicate is returned as-is.
	if doc.Status == "queued" {
		go h.store.RunPipeline(context.Background(), doc.ID)
	}

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

func (h DocumentsHandler) ListPassages(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	items, err := h.store.ListPassages(r.Context(), documentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list passages"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h DocumentsHandler) ListSentences(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	passageID := r.URL.Query().Get("passageId")
	items, err := h.store.ListSentences(r.Context(), documentID, passageID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list sentences"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h DocumentsHandler) ListPipelineJobs(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	items, err := h.store.ListPipelineJobs(r.Context(), documentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list pipeline jobs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h DocumentsHandler) ListConcordance(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	query := r.URL.Query()
	limit := parseIntWithDefault(query.Get("limit"), 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := max(parseIntWithDefault(query.Get("offset"), 0), 0)

	filter := documents.ConcordanceFilter{
		Lemma:   strings.TrimSpace(query.Get("lemma")),
		POS:     strings.TrimSpace(query.Get("pos")),
		Section: strings.TrimSpace(query.Get("section")),
		Limit:   limit,
		Offset:  offset,
		SortBy:  strings.TrimSpace(query.Get("sort")),
		SortDir: strings.TrimSpace(query.Get("dir")),
	}

	total, err := h.store.CountConcordance(r.Context(), documentID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to count concordance"})
		return
	}

	items, err := h.store.ListConcordance(r.Context(), documentID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list concordance"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (h DocumentsHandler) GetDocumentStats(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	if _, ok := h.store.Get(r.Context(), documentID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	stats, err := h.store.Stats(r.Context(), documentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to load document stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h DocumentsHandler) RenameDocument(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FileName string `json:"fileName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	doc, ok, err := h.store.Rename(r.Context(), r.PathValue("documentId"), input.FileName)
	if errors.Is(err, documents.ErrInvalidFileName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to rename document"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (h DocumentsHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	doc, ok, err := h.store.Delete(r.Context(), r.PathValue("documentId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to delete document"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	if doc.LocalPath != "" {
		_ = os.Remove(doc.LocalPath)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h DocumentsHandler) ListKWIC(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	filter := kwicFilterFromQuery(r)

	total, err := h.store.CountKWIC(r.Context(), documentID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to count kwic occurrences"})
		return
	}

	items, err := h.store.ListKWIC(r.Context(), documentID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list kwic occurrences"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
		"sort":   filter.SortBy,
		"dir":    filter.SortDir,
	})
}

func (h DocumentsHandler) RetryDocument(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	doc, ok, err := h.store.Retry(r.Context(), documentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to queue retry"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	go h.store.RunPipeline(context.Background(), doc.ID)

	writeJSON(w, http.StatusAccepted, doc)
}

func saveUploadFile(uploadDir, fileName string, reader io.Reader) (string, string, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", "", err
	}

	targetPath := uniqueUploadPath(uploadDir, fileName)
	output, err := os.Create(targetPath)
	if err != nil {
		return "", "", err
	}
	defer output.Close()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)
	if _, err := io.Copy(output, tee); err != nil {
		return "", "", err
	}

	return targetPath, hex.EncodeToString(hasher.Sum(nil)), nil
}

func uniqueUploadPath(uploadDir, fileName string) string {
	base := filepath.Base(fileName)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "document"
	}

	stamp := time.Now().UTC().Format("20060102T150405.000000000")
	return filepath.Join(uploadDir, name+"-"+stamp+ext)
}

func detectFormat(fileName string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	if ext == "" {
		return "txt"
	}

	return ext
}

func parseIntWithDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return parsed
}

func normalizeKWICPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}

func normalizeKWICSortBy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "lemma":
		return "lemma"
	case "keyword":
		return "keyword"
	default:
		return "position"
	}
}

func normalizeSortDir(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "desc") {
		return "desc"
	}

	return "asc"
}
