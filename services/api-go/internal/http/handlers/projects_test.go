package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"concordance/services/api-go/internal/projects"
)

func TestCreateProject(t *testing.T) {
	store := projects.NewMemoryStore()
	h := NewProjectsHandler(store)

	body := []byte(`{"name":"Shakespeare"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.CreateProject(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}
}

func TestListProjects(t *testing.T) {
	store := projects.NewMemoryStore()
	_, _ = store.Create(context.Background(), projects.CreateProjectInput{Name: "Poetry"})
	h := NewProjectsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rr := httptest.NewRecorder()

	h.ListProjects(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d", rr.Code)
	}

	var payload map[string][]projects.Project
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unable to parse response: %v", err)
	}

	if len(payload["items"]) != 1 {
		t.Fatalf("unexpected project count: got=%d", len(payload["items"]))
	}
}
