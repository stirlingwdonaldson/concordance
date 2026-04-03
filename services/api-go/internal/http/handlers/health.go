package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"concordance/services/api-go/internal/config"
)

type HealthHandler struct {
	cfg      config.Config
	checkers ReadinessChecks
}

type ReadinessChecks struct {
	Database func(ctx context.Context) error
}

type HealthResponse struct {
	Service         string `json:"service"`
	Status          string `json:"status"`
	PipelineVersion string `json:"pipelineVersion"`
	EmbeddingModel  string `json:"embeddingModel"`
	EmbeddingDim    int    `json:"embeddingDim"`
}

func NewHealthHandler(cfg config.Config, checks ReadinessChecks) HealthHandler {
	return HealthHandler{cfg: cfg, checkers: checks}
}

func (h HealthHandler) GetHealth(w http.ResponseWriter, _ *http.Request) {
	resp := HealthResponse{
		Service:         "api-go",
		Status:          "ok",
		PipelineVersion: h.cfg.PipelineVersion,
		EmbeddingModel:  h.cfg.EmbeddingModel,
		EmbeddingDim:    h.cfg.EmbeddingDim,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h HealthHandler) GetReady(w http.ResponseWriter, r *http.Request) {
	databaseStatus := "ready"
	readyStatus := "ready"
	if h.checkers.Database != nil {
		if err := h.checkers.Database(r.Context()); err != nil {
			databaseStatus = "failed"
			readyStatus = "degraded"
		}
	}

	resp := map[string]any{
		"service": "api-go",
		"status":  readyStatus,
		"checks": map[string]string{
			"database":    databaseStatus,
			"nlp_sidecar": "pending",
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
