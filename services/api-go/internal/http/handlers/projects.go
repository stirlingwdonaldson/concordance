package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"concordance/services/api-go/internal/projects"
)

type ProjectsHandler struct {
	store projects.Store
}

func NewProjectsHandler(store projects.Store) ProjectsHandler {
	return ProjectsHandler{store: store}
}

func (h ProjectsHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var input projects.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	project, err := h.store.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, projects.ErrInvalidName) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to create project"})
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (h ProjectsHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	allProjects, err := h.store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to list projects"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": allProjects})
}
