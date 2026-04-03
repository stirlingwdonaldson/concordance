package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"concordance/services/api-go/internal/config"
)

func TestGetHealth(t *testing.T) {
	h := NewHealthHandler(config.Config{
		PipelineVersion: "v-test",
		EmbeddingModel:  "all-MiniLM-L6-v2",
		EmbeddingDim:    384,
	}, ReadinessChecks{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.GetHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response payload: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("unexpected status payload: got=%s", resp.Status)
	}

	if resp.EmbeddingModel != "all-MiniLM-L6-v2" {
		t.Fatalf("unexpected embedding model: got=%s", resp.EmbeddingModel)
	}
}

func TestGetReadyDatabaseFailed(t *testing.T) {
	h := NewHealthHandler(config.Config{}, ReadinessChecks{
		Database: func(_ context.Context) error {
			return context.DeadlineExceeded
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	h.GetReady(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid response payload: %v", err)
	}

	if payload["status"] != "degraded" {
		t.Fatalf("unexpected readiness status: got=%v", payload["status"])
	}
}
