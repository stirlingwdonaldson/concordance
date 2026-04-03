package router

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"concordance/services/api-go/internal/http/handlers"
)

func New(health handlers.HealthHandler, projects handlers.ProjectsHandler, documents handlers.DocumentsHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.GetHealth)
	mux.HandleFunc("GET /ready", health.GetReady)
	mux.HandleFunc("GET /api/projects", projects.ListProjects)
	mux.HandleFunc("POST /api/projects", projects.CreateProject)
	mux.HandleFunc("POST /api/projects/{projectId}/documents/upload", documents.UploadDocument)
	mux.HandleFunc("GET /api/projects/{projectId}/documents", documents.ListProjectDocuments)
	mux.HandleFunc("GET /api/documents/{documentId}/status", documents.GetDocumentStatus)
	mux.HandleFunc("POST /api/documents/{documentId}/retry", documents.RetryDocument)
	mux.HandleFunc("GET /api/documents/{documentId}/passages", documents.ListPassages)
	mux.HandleFunc("GET /api/documents/{documentId}/sentences", documents.ListSentences)
	mux.HandleFunc("GET /api/documents/{documentId}/pipeline-jobs", documents.ListPipelineJobs)
	mux.HandleFunc("GET /ws/jobs", documents.StreamJobs)

	return withRequestLog(withCORS(mux))
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s duration_ms=%d", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := isAllowedOrigin(origin)

		if allowed {
			setCORSHeaders(w, origin)
		}

		if r.Method == http.MethodOptions {
			if origin != "" && !allowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Add("Vary", "Origin")
	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
}

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
