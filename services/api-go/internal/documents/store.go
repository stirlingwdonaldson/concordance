package documents

import "context"

type Store interface {
	Create(ctx context.Context, input CreateInput) (Document, error)
	Get(ctx context.Context, documentID string) (Document, bool)
	ListByProject(ctx context.Context, projectID string) ([]Document, error)
	ListPassages(ctx context.Context, documentID string) ([]Passage, error)
	ListSentences(ctx context.Context, documentID, passageID string) ([]Sentence, error)
	ListPipelineJobs(ctx context.Context, documentID string) ([]PipelineJob, error)
	RunPipeline(ctx context.Context, documentID string)
}
