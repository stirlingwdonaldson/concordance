package documents

import (
	"strings"
	"testing"
)

func sentenceRows(passageSizes ...int) []nlpSentenceRow {
	rows := make([]nlpSentenceRow, 0)
	for passage, count := range passageSizes {
		for i := 0; i < count; i++ {
			rows = append(rows, nlpSentenceRow{
				PassageID: string(rune('A' + passage)),
				ID:        string(rune('a'+passage)) + strings.Repeat("x", i),
				Text:      "0123456789", // 10 bytes
			})
		}
	}

	return rows
}

func TestBuildAnalyzeBatchesEmptyInput(t *testing.T) {
	if batches := buildAnalyzeBatches(nil, 100, 100); len(batches) != 0 {
		t.Fatalf("expected no batches, got %d", len(batches))
	}
}

func TestBuildAnalyzeBatchesKeepsEverythingInOrderAndWithinBudget(t *testing.T) {
	rows := sentenceRows(7, 3, 9) // 19 sentences, 10 bytes each

	// Budget of 35 bytes = 3 sentences per batch.
	batches := buildAnalyzeBatches(rows, 35, 1000)

	var gotIDs []string
	for i, batch := range batches {
		chars, sentences := 0, 0
		for _, passage := range batch {
			for _, sentence := range passage.Sentences {
				chars += len(sentence.Text)
				sentences++
				gotIDs = append(gotIDs, passage.PassageId+"/"+sentence.SentenceId)
			}
		}
		if chars > 35 {
			t.Fatalf("batch %d exceeds char budget: %d", i, chars)
		}
		if sentences == 0 {
			t.Fatalf("batch %d is empty", i)
		}
	}

	if len(gotIDs) != len(rows) {
		t.Fatalf("lost or duplicated sentences: got=%d want=%d", len(gotIDs), len(rows))
	}
	for i, row := range rows {
		if want := row.PassageID + "/" + row.ID; gotIDs[i] != want {
			t.Fatalf("order changed at %d: got=%s want=%s", i, gotIDs[i], want)
		}
	}

	if want := 7; len(batches) != want { // ceil(19/3)
		t.Fatalf("unexpected batch count: got=%d want=%d", len(batches), want)
	}
}

func TestBuildAnalyzeBatchesSplitsOnePassageAcrossBatches(t *testing.T) {
	rows := sentenceRows(10) // one passage, 100 bytes

	batches := buildAnalyzeBatches(rows, 40, 1000)
	if len(batches) != 3 {
		t.Fatalf("expected the passage to be split into 3 batches, got %d", len(batches))
	}

	for i, batch := range batches {
		if len(batch) != 1 || batch[0].PassageId != "A" {
			t.Fatalf("batch %d should hold a slice of passage A", i)
		}
	}
}

func TestBuildAnalyzeBatchesHonorsSentenceLimit(t *testing.T) {
	rows := sentenceRows(25)

	batches := buildAnalyzeBatches(rows, 1_000_000, 10)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches of at most 10 sentences, got %d", len(batches))
	}
}

func TestBuildAnalyzeBatchesGivesOversizedSentenceItsOwnBatch(t *testing.T) {
	rows := sentenceRows(3)
	rows[1].Text = strings.Repeat("y", 500)

	batches := buildAnalyzeBatches(rows, 50, 1000)

	total := 0
	foundBig := false
	for _, batch := range batches {
		for _, passage := range batch {
			for _, sentence := range passage.Sentences {
				total++
				if len(sentence.Text) == 500 {
					foundBig = true
				}
			}
		}
	}

	if total != 3 || !foundBig {
		t.Fatalf("oversized sentence was dropped: total=%d foundBig=%v", total, foundBig)
	}
}
