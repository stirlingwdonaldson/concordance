package documents

import (
	"context"
	"errors"
)

var ErrInvalidFileName = errors.New("document name is required")

type Store interface {
	Create(ctx context.Context, input CreateInput) (Document, error)
	Get(ctx context.Context, documentID string) (Document, bool)
	ListByProject(ctx context.Context, projectID string) ([]Document, error)
	ListPassages(ctx context.Context, documentID string) ([]Passage, error)
	ListSentences(ctx context.Context, documentID, passageID string) ([]Sentence, error)
	ListPipelineJobs(ctx context.Context, documentID string) ([]PipelineJob, error)
	ListConcordance(ctx context.Context, documentID string, filter ConcordanceFilter) ([]ConcordanceTerm, error)
	ListKWIC(ctx context.Context, documentID string, filter KWICFilter) ([]KWICOccurrence, error)
	CountKWIC(ctx context.Context, documentID string, filter KWICFilter) (int, error)
	CountConcordance(ctx context.Context, documentID string, filter ConcordanceFilter) (int, error)
	Stats(ctx context.Context, documentID string) (Stats, error)
	Rename(ctx context.Context, documentID, fileName string) (Document, bool, error)
	Delete(ctx context.Context, documentID string) (Document, bool, error)
	PartsOfSpeech(ctx context.Context, documentID string) ([]POSCount, error)
	ListProjectKWIC(ctx context.Context, projectID string, filter KWICFilter) ([]KWICOccurrence, error)
	CountProjectKWIC(ctx context.Context, projectID string, filter KWICFilter) (int, error)
	Keyness(ctx context.Context, documentID string, filter KeynessFilter) ([]KeyTerm, int, error)
	Retry(ctx context.Context, documentID string) (Document, bool, error)
	RunPipeline(ctx context.Context, documentID string)
}
