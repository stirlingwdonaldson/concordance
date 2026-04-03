package handlers

import (
	"encoding/json"
	"net/http"

	"concordance/services/api-go/internal/config"
)

type HealthHandler struct {
	cfg config.Config
}

type HealthResponse struct {
	Service         string `json:"service"`
	Status          string `json:"status"`
	PipelineVersion string `json:"pipelineVersion"`
	EmbeddingModel  string `json:"embeddingModel"`
	EmbeddingDim    int    `json:"embeddingDim"`
}

func NewHealthHandler(cfg config.Config) HealthHandler {
	return HealthHandler{cfg: cfg}
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

func (h HealthHandler) GetReady(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"service": "api-go",
		"status":  "ready",
		"checks": map[string]string{
			"database":    "pending",
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
