package documents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

var stages = []string{
	"ingest_parse",
	"segment_structure",
	"nlp_analyze",
	"concordance_aggregate",
	"finalize_ready",
}

type MemoryStore struct {
	mu        sync.RWMutex
	documents map[string]Document
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{documents: make(map[string]Document)}
}

func (s *MemoryStore) Create(_ context.Context, projectID, fileName, localPath string) (Document, error) {
	now := time.Now().UTC()
	doc := Document{
		ID:        randomID(),
		ProjectID: projectID,
		FileName:  fileName,
		LocalPath: localPath,
		Status:    "queued",
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.documents[doc.ID] = doc
	s.mu.Unlock()

	return doc, nil
}

func (s *MemoryStore) Get(_ context.Context, documentID string) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[documentID]
	return doc, ok
}

func (s *MemoryStore) ListByProject(_ context.Context, projectID string) ([]Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Document, 0)
	for _, doc := range s.documents {
		if doc.ProjectID == projectID {
			out = append(out, doc)
		}
	}

	return out, nil
}

func (s *MemoryStore) RunPipeline(ctx context.Context, documentID string) {
	for idx, stageName := range stages {
		select {
		case <-ctx.Done():
			s.markFailed(documentID)
			return
		default:
		}

		s.markStage(documentID, stageName, float64(idx+1)/float64(len(stages)))
		time.Sleep(150 * time.Millisecond)
	}
}

func (s *MemoryStore) markStage(documentID, stageName string, progress float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.documents[documentID]
	if !ok {
		return
	}

	doc.Status = stageName
	doc.Progress = progress
	doc.UpdatedAt = time.Now().UTC()
	if progress >= 1 {
		doc.Status = "ready"
	}
	s.documents[documentID] = doc
}

func (s *MemoryStore) markFailed(documentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.documents[documentID]
	if !ok {
		return
	}

	doc.Status = "failed"
	doc.UpdatedAt = time.Now().UTC()
	s.documents[documentID] = doc
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
