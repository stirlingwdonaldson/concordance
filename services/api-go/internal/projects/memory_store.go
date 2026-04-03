package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrInvalidName = errors.New("project name is required")

type MemoryStore struct {
	mu       sync.RWMutex
	projects []Project
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Create(_ context.Context, input CreateProjectInput) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, ErrInvalidName
	}

	now := time.Now().UTC()
	project := Project{
		ID:          randomID(),
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects = append(s.projects, project)

	return project, nil
}

func (s *MemoryStore) List(_ context.Context) ([]Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Project, len(s.projects))
	copy(out, s.projects)

	return out, nil
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
