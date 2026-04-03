package documents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.RWMutex
	documents map[string]Document
	events    *JobEventBroker
}

func NewMemoryStore(events *JobEventBroker) *MemoryStore {
	return &MemoryStore{documents: make(map[string]Document), events: events}
}

func (s *MemoryStore) Create(_ context.Context, input CreateInput) (Document, error) {
	now := time.Now().UTC()
	doc := Document{
		ID:         randomID(),
		ProjectID:  input.ProjectID,
		FileName:   input.FileName,
		LocalPath:  input.LocalPath,
		SourceHash: input.SourceHash,
		Format:     input.Format,
		Status:     "queued",
		Progress:   0,
		CreatedAt:  now,
		UpdatedAt:  now,
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

func (s *MemoryStore) ListPassages(_ context.Context, _ string) ([]Passage, error) {
	return []Passage{}, nil
}

func (s *MemoryStore) ListSentences(_ context.Context, _, _ string) ([]Sentence, error) {
	return []Sentence{}, nil
}

func (s *MemoryStore) ListPipelineJobs(_ context.Context, _ string) ([]PipelineJob, error) {
	return []PipelineJob{}, nil
}

func (s *MemoryStore) Retry(_ context.Context, documentID string) (Document, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.documents[documentID]
	if !ok {
		return Document{}, false, nil
	}

	doc.Status = "queued"
	doc.Progress = 0
	doc.UpdatedAt = time.Now().UTC()
	s.documents[documentID] = doc

	return doc, true, nil
}

func (s *MemoryStore) RunPipeline(ctx context.Context, documentID string) {
	doc, ok := s.Get(ctx, documentID)
	if !ok {
		return
	}

	s.publishDocumentStatus(doc.ID, doc.ProjectID, doc.Status, doc.Progress, "")

	for idx, stageName := range stageOrder {
		select {
		case <-ctx.Done():
			s.markFailed(documentID)
			s.publishDocumentStatus(doc.ID, doc.ProjectID, "failed", float64(idx)/float64(len(stageOrder)), "pipeline canceled")
			return
		default:
		}

		progress := float64(idx+1) / float64(len(stageOrder))
		startedAt := time.Now().UTC()
		s.markStage(documentID, stageName, progress)
		s.publishEvent(NewJobEvent("stage_started", doc.ID, doc.ProjectID, map[string]any{
			"stage":    stageName,
			"status":   "running",
			"progress": progress,
			"message":  "processing stage",
		}))
		s.publishDocumentStatus(doc.ID, doc.ProjectID, stageName, progress, "")
		time.Sleep(150 * time.Millisecond)

		finishedAt := time.Now().UTC()
		s.publishEvent(NewJobEvent("stage_completed", doc.ID, doc.ProjectID, map[string]any{
			"stage":      stageName,
			"status":     "completed",
			"progress":   progress,
			"message":    "stage complete",
			"startedAt":  startedAt,
			"finishedAt": finishedAt,
		}))
	}

	s.publishDocumentStatus(doc.ID, doc.ProjectID, "ready", 1, "")
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

func (s *MemoryStore) publishDocumentStatus(documentID, projectID, status string, progress float64, ingestError string) {
	payload := map[string]any{
		"status":   status,
		"progress": progress,
	}
	if ingestError != "" {
		payload["ingestError"] = ingestError
	}

	s.publishEvent(NewJobEvent("document_status", documentID, projectID, payload))
}

func (s *MemoryStore) publishEvent(event JobEvent) {
	if s.events == nil {
		return
	}

	s.events.Publish(event)
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
