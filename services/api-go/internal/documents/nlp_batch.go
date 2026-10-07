package documents

import "concordance/services/api-go/internal/nlpv1"

const (
	// maxBatchChars and maxBatchSentences bound how much text is sent to the NLP
	// sidecar in a single AnalyzeDocument call. The tokenized response is roughly
	// 10x the size of the input text, so these keep each response comfortably
	// small regardless of how large the document is.
	maxBatchChars     = 200_000
	maxBatchSentences = 5_000
)

// buildAnalyzeBatches groups ordered sentence rows into request-sized batches of
// passage inputs. A passage that is too large for one batch is split across
// several batches (tokens are keyed by sentence ID, so this is safe). A single
// sentence larger than the budget still gets a batch of its own rather than
// being dropped.
func buildAnalyzeBatches(rows []nlpSentenceRow, maxChars, maxSentences int) [][]*nlpv1.PassageInput {
	if maxChars <= 0 {
		maxChars = maxBatchChars
	}
	if maxSentences <= 0 {
		maxSentences = maxBatchSentences
	}

	batches := make([][]*nlpv1.PassageInput, 0)

	var (
		current   []*nlpv1.PassageInput
		passage   *nlpv1.PassageInput
		charCount int
		sentCount int
	)

	flush := func() {
		if len(current) > 0 {
			batches = append(batches, current)
		}
		current = nil
		passage = nil
		charCount = 0
		sentCount = 0
	}

	for _, row := range rows {
		if sentCount > 0 && (charCount+len(row.Text) > maxChars || sentCount+1 > maxSentences) {
			flush()
		}

		if passage == nil || passage.PassageId != row.PassageID {
			passage = &nlpv1.PassageInput{PassageId: row.PassageID}
			current = append(current, passage)
		}

		passage.Sentences = append(passage.Sentences, &nlpv1.SentenceInput{
			SentenceId: row.ID,
			Text:       row.Text,
			StartChar:  row.StartChar,
			EndChar:    row.EndChar,
		})
		charCount += len(row.Text)
		sentCount++
	}

	flush()

	return batches
}
