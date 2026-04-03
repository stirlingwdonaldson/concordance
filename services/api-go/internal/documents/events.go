package documents

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type JobEvent struct {
	Version    string         `json:"version"`
	EventID    string         `json:"eventId"`
	EventType  string         `json:"eventType"`
	OccurredAt time.Time      `json:"occurredAt"`
	DocumentID string         `json:"documentId,omitempty"`
	ProjectID  string         `json:"projectId,omitempty"`
	Payload    map[string]any `json:"payload"`
}

type JobEventFilter struct {
	DocumentID string
	ProjectID  string
}

type JobEventBroker struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]eventSubscription
}

type eventSubscription struct {
	filter JobEventFilter
	ch     chan JobEvent
}

func NewJobEventBroker() *JobEventBroker {
	return &JobEventBroker{subscribers: make(map[int]eventSubscription)}
}

func (b *JobEventBroker) Subscribe(filter JobEventFilter) (<-chan JobEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	ch := make(chan JobEvent, 32)
	b.subscribers[id] = eventSubscription{filter: filter, ch: ch}

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		sub, ok := b.subscribers[id]
		if !ok {
			return
		}

		delete(b.subscribers, id)
		close(sub.ch)
	}

	return ch, unsubscribe
}

func (b *JobEventBroker) Publish(event JobEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, sub := range b.subscribers {
		if !sub.filter.matches(event) {
			continue
		}

		select {
		case sub.ch <- event:
		default:
			delete(b.subscribers, id)
			close(sub.ch)
		}
	}
}

func (f JobEventFilter) matches(event JobEvent) bool {
	if f.DocumentID != "" && event.DocumentID != f.DocumentID {
		return false
	}

	if f.ProjectID != "" && event.ProjectID != f.ProjectID {
		return false
	}

	return true
}

func NewJobEvent(eventType, documentID, projectID string, payload map[string]any) JobEvent {
	if payload == nil {
		payload = map[string]any{}
	}

	return JobEvent{
		Version:    "v1",
		EventID:    uuid.NewString(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		DocumentID: documentID,
		ProjectID:  projectID,
		Payload:    payload,
	}
}
