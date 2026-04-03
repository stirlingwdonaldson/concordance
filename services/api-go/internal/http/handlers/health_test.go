package handlers

import (
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
	})

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
