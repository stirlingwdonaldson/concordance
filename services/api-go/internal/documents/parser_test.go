package documents

import "testing"

func TestSplitParagraphs(t *testing.T) {
	text := "First paragraph.\nStill first.\n\nSecond paragraph."
	segments := splitParagraphs(text)

	if len(segments) != 2 {
		t.Fatalf("unexpected paragraph count: got=%d", len(segments))
	}

	if segments[0].Text != "First paragraph.\nStill first." {
		t.Fatalf("unexpected first paragraph text: got=%q", segments[0].Text)
	}
}

func TestSplitSentences(t *testing.T) {
	passage := passageSegment{Text: "Call me Ishmael. Some years ago.", StartChar: 10, EndChar: 42}
	sentences := splitSentences(passage)

	if len(sentences) != 2 {
		t.Fatalf("unexpected sentence count: got=%d", len(sentences))
	}

	if sentences[0].Text != "Call me Ishmael." {
		t.Fatalf("unexpected first sentence: got=%q", sentences[0].Text)
	}

	if sentences[0].StartChar != 10 {
		t.Fatalf("unexpected first sentence start: got=%d", sentences[0].StartChar)
	}
}
