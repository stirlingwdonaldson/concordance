package documents

import "testing"

func TestJobEventFilterMatches(t *testing.T) {
	filter := JobEventFilter{DocumentID: "d1", ProjectID: "p1"}
	event := NewJobEvent("document_status", "d1", "p1", nil)

	if !filter.matches(event) {
		t.Fatalf("expected filter to match event")
	}

	if filter.matches(NewJobEvent("document_status", "d2", "p1", nil)) {
		t.Fatalf("expected filter to reject non-matching document")
	}

	if filter.matches(NewJobEvent("document_status", "d1", "p2", nil)) {
		t.Fatalf("expected filter to reject non-matching project")
	}
}

func TestJobEventBrokerPublishesToMatchingSubscribers(t *testing.T) {
	broker := NewJobEventBroker()
	matching, unsubscribeMatching := broker.Subscribe(JobEventFilter{DocumentID: "d1"})
	defer unsubscribeMatching()

	nonMatching, unsubscribeNonMatching := broker.Subscribe(JobEventFilter{DocumentID: "d2"})
	defer unsubscribeNonMatching()

	event := NewJobEvent("stage_started", "d1", "p1", map[string]any{"stage": "ingest_parse"})
	broker.Publish(event)

	select {
	case received := <-matching:
		if received.DocumentID != "d1" {
			t.Fatalf("unexpected document id: got=%s", received.DocumentID)
		}
	default:
		t.Fatalf("expected matching subscriber to receive event")
	}

	select {
	case <-nonMatching:
		t.Fatalf("expected non-matching subscriber to not receive event")
	default:
	}
}
