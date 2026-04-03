package router

import (
	"log"
	"net/http"
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

	return withRequestLog(mux)
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s duration_ms=%d", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}
