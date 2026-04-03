package documents

import "context"

type Store interface {
	Create(ctx context.Context, input CreateInput) (Document, error)
	Get(ctx context.Context, documentID string) (Document, bool)
	ListByProject(ctx context.Context, projectID string) ([]Document, error)
	RunPipeline(ctx context.Context, documentID string)
}
