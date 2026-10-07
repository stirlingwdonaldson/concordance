package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/projects"
)

type ProjectsHandler struct {
	store projects.Store
	// documents lets project deletion also remove the uploaded files.
	documents documents.Store
}

// WithDocuments returns a copy that cleans up uploaded files on delete.
func (h ProjectsHandler) WithDocuments(store documents.Store) ProjectsHandler {
	h.documents = store
	return h
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

func (h ProjectsHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var input projects.UpdateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	project, ok, err := h.store.Update(r.Context(), r.PathValue("projectId"), input)
	if errors.Is(err, projects.ErrInvalidName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to update project"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h ProjectsHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	var files []string
	if h.documents != nil {
		if docs, err := h.documents.ListByProject(r.Context(), projectID); err == nil {
			for _, doc := range docs {
				files = append(files, doc.LocalPath)
			}
		}
	}

	ok, err := h.store.Delete(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unable to delete project"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	for _, path := range files {
		if path != "" {
			_ = os.Remove(path)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
