package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"concordance/services/api-go/internal/documents"
)

// Exports stop here so a request can't build an unbounded file.
const (
	maxKWICExportRows = 50_000
	maxWordExportRows = 200_000
	exportPageSize    = 500
)

func (h DocumentsHandler) ListPartsOfSpeech(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.PartsOfSpeech(r.Context(), r.PathValue("documentId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list parts of speech"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func kwicFilterFromQuery(r *http.Request) documents.KWICFilter {
	query := r.URL.Query()
	limit, offset := normalizeKWICPage(
		parseIntWithDefault(query.Get("limit"), 50),
		parseIntWithDefault(query.Get("offset"), 0),
	)

	return documents.KWICFilter{
		Lemma:   strings.TrimSpace(query.Get("lemma")),
		POS:     strings.TrimSpace(query.Get("pos")),
		Page:    strings.TrimSpace(query.Get("page")),
		Section: strings.TrimSpace(query.Get("section")),
		Limit:   limit,
		Offset:  offset,
		SortBy:  normalizeKWICSortBy(query.Get("sort")),
		SortDir: normalizeSortDir(query.Get("dir")),
	}
}

// ListProjectKWIC searches every file in a project at once.
func (h DocumentsHandler) ListProjectKWIC(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	filter := kwicFilterFromQuery(r)

	total, err := h.store.CountProjectKWIC(r.Context(), projectID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to count matches"})
		return
	}
	items, err := h.store.ListProjectKWIC(r.Context(), projectID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to search project"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": filter.Limit, "offset": filter.Offset,
	})
}

type compareResponse struct {
	Items     []documents.KeyTerm `json:"items"`
	Total     int                 `json:"total"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
	DocumentA string              `json:"documentA"`
	DocumentB string              `json:"documentB"`
}

// keynessRequest validates the two documents and reads the filter. On failure it
// has already written the error response.
func (h DocumentsHandler) keynessRequest(w http.ResponseWriter, r *http.Request) (documents.Document, documents.Document, documents.KeynessFilter, bool) {
	query := r.URL.Query()
	a, okA := h.store.Get(r.Context(), r.PathValue("documentId"))
	b, okB := h.store.Get(r.Context(), strings.TrimSpace(query.Get("against")))

	switch {
	case !okA:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
	case strings.TrimSpace(query.Get("against")) == "":
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose a file to compare against"})
	case !okB:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "the file to compare against was not found"})
	case a.ID == b.ID:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose a different file to compare against"})
	case a.Status != "ready" || b.Status != "ready":
		writeJSON(w, http.StatusConflict, map[string]string{"error": "both files must finish processing before they can be compared"})
	default:
		side := strings.ToLower(strings.TrimSpace(query.Get("side")))
		if side != "a" && side != "b" {
			side = ""
		}
		limit := min(max(parseIntWithDefault(query.Get("limit"), 25), 1), 500)

		return a, b, documents.KeynessFilter{
			Against: b.ID,
			POS:     strings.TrimSpace(query.Get("pos")),
			Lemma:   strings.TrimSpace(query.Get("lemma")),
			Side:    side,
			MinFreq: max(parseIntWithDefault(query.Get("min"), 5), 1),
			Limit:   limit,
			Offset:  max(parseIntWithDefault(query.Get("offset"), 0), 0),
		}, true
	}

	return documents.Document{}, documents.Document{}, documents.KeynessFilter{}, false
}

func (h DocumentsHandler) CompareDocuments(w http.ResponseWriter, r *http.Request) {
	a, b, filter, ok := h.keynessRequest(w, r)
	if !ok {
		return
	}

	items, total, err := h.store.Keyness(r.Context(), a.ID, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to compare files"})
		return
	}

	writeJSON(w, http.StatusOK, compareResponse{
		Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset,
		DocumentA: a.FileName, DocumentB: b.FileName,
	})
}

// ---- CSV exports ----

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func exportName(base, suffix string) string {
	base = strings.TrimSuffix(base, ".pdf")
	name := strings.Trim(unsafeFileChars.ReplaceAllString(base, "_"), "_.")
	if name == "" {
		name = "export"
	}

	return name + "-" + suffix + ".csv"
}

// startCSV writes the headers and a UTF-8 byte-order mark so Excel opens accented text correctly.
func startCSV(w http.ResponseWriter, filename string, header []string) *csv.Writer {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	_, _ = w.Write([]byte("\xef\xbb\xbf"))
	out := csv.NewWriter(w)
	_ = out.Write(header)

	return out
}

// safeCell stops spreadsheets from running text taken from a document as a formula.
func safeCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}

	return value
}

func (h DocumentsHandler) ExportConcordance(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	doc, ok := h.store.Get(r.Context(), documentID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	stats, err := h.store.Stats(r.Context(), documentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to export word list"})
		return
	}

	query := r.URL.Query()
	filter := documents.ConcordanceFilter{
		Lemma:   strings.TrimSpace(query.Get("lemma")),
		POS:     strings.TrimSpace(query.Get("pos")),
		SortBy:  strings.TrimSpace(query.Get("sort")),
		SortDir: strings.TrimSpace(query.Get("dir")),
		Limit:   exportPageSize * 10,
	}

	out := startCSV(w, exportName(doc.FileName, "word-list"), []string{"word", "count", "per_1000_words", "appears_once"})
	for written := 0; written < maxWordExportRows; {
		filter.Offset = written
		page, err := h.store.ListConcordance(r.Context(), documentID, filter)
		if err != nil || len(page) == 0 {
			break
		}
		for _, term := range page {
			per1000 := 0.0
			if stats.Tokens > 0 {
				per1000 = float64(term.TotalFreq) / float64(stats.Tokens) * 1000
			}
			_ = out.Write([]string{safeCell(term.Lemma), strconv.Itoa(term.TotalFreq), strconv.FormatFloat(per1000, 'f', 3, 64), strconv.FormatBool(term.Hapax)})
		}
		written += len(page)
	}
	out.Flush()
}

func (h DocumentsHandler) writeKWICCSV(w http.ResponseWriter, r *http.Request, filename string, withDocument bool, list func(documents.KWICFilter) ([]documents.KWICOccurrence, error)) {
	filter := kwicFilterFromQuery(r)
	filter.Limit = exportPageSize

	header := []string{"left_context", "keyword", "right_context", "word", "part_of_speech"}
	if withDocument {
		header = append([]string{"file"}, header...)
	}
	out := startCSV(w, filename, header)
	for written := 0; written < maxKWICExportRows; {
		filter.Offset = written
		page, err := list(filter)
		if err != nil || len(page) == 0 {
			break
		}
		for _, row := range page {
			cells := []string{safeCell(row.LeftContext), safeCell(row.Keyword), safeCell(row.RightContext), safeCell(row.Lemma), row.POS}
			if withDocument {
				cells = append([]string{safeCell(row.DocumentName)}, cells...)
			}
			_ = out.Write(cells)
		}
		written += len(page)
	}
	out.Flush()
}

func (h DocumentsHandler) ExportKWIC(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	doc, ok := h.store.Get(r.Context(), documentID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	h.writeKWICCSV(w, r, exportName(doc.FileName, "in-context"), false, func(f documents.KWICFilter) ([]documents.KWICOccurrence, error) {
		return h.store.ListKWIC(r.Context(), documentID, f)
	})
}

func (h DocumentsHandler) ExportProjectKWIC(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	h.writeKWICCSV(w, r, "project-in-context.csv", true, func(f documents.KWICFilter) ([]documents.KWICOccurrence, error) {
		return h.store.ListProjectKWIC(r.Context(), projectID, f)
	})
}

func (h DocumentsHandler) ExportComparison(w http.ResponseWriter, r *http.Request) {
	a, b, filter, ok := h.keynessRequest(w, r)
	if !ok {
		return
	}
	filter.Limit = exportPageSize

	out := startCSV(w, exportName(a.FileName, "vs-"+strings.TrimSuffix(b.FileName, ".pdf")), []string{
		"word", "count_in_" + safeHeader(a.FileName), "count_in_" + safeHeader(b.FileName),
		"per_million_a", "per_million_b", "score", "more_frequent_in",
	})
	for written := 0; written < maxKWICExportRows; {
		filter.Offset = written
		items, _, err := h.store.Keyness(r.Context(), a.ID, filter)
		if err != nil || len(items) == 0 {
			break
		}
		for _, item := range items {
			winner := a.FileName
			if item.Side == "b" {
				winner = b.FileName
			}
			_ = out.Write([]string{
				safeCell(item.Lemma), strconv.Itoa(item.FreqA), strconv.Itoa(item.FreqB),
				strconv.FormatFloat(item.PerMillionA, 'f', 1, 64), strconv.FormatFloat(item.PerMillionB, 'f', 1, 64),
				strconv.FormatFloat(item.Score, 'f', 2, 64), safeCell(winner),
			})
		}
		written += len(items)
	}
	out.Flush()
}

func safeHeader(name string) string {
	return strings.Trim(unsafeFileChars.ReplaceAllString(name, "_"), "_")
}
