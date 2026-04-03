package nlp

import (
	"context"

	"concordance/services/api-go/internal/nlpv1"
)

type Client interface {
	Health(ctx context.Context) (*nlpv1.HealthResponse, error)
	AnalyzeDocument(ctx context.Context, req *nlpv1.AnalyzeDocumentRequest) (*nlpv1.AnalyzeDocumentResponse, error)
	Close() error
}
