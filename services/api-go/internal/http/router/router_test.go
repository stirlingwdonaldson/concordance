package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"concordance/services/api-go/internal/config"
	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/http/handlers"
	"concordance/services/api-go/internal/projects"
)

func TestCORSAllowsLocalhostOrigin(t *testing.T) {
	h := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected access-control-allow-origin header, got %q", got)
	}
}

func TestCORSPreflightAllowsPOST(t *testing.T) {
	h := newTestRouter()
	req := httptest.NewRequest(http.MethodOptions, "/api/projects", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("expected access-control-allow-origin header, got %q", got)
	}
}

func TestCORSBlocksUnknownOriginPreflight(t *testing.T) {
	h := newTestRouter()
	req := httptest.NewRequest(http.MethodOptions, "/api/projects", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no access-control-allow-origin header, got %q", got)
	}
}

func newTestRouter() http.Handler {
	health := handlers.NewHealthHandler(config.Config{}, handlers.ReadinessChecks{
		Database: func(context.Context) error { return nil },
	})
	projectHandlers := handlers.NewProjectsHandler(projects.NewMemoryStore())
	documentHandlers := handlers.NewDocumentsHandler(documents.NewMemoryStore(), "")

	return New(health, projectHandlers, documentHandlers)
}
