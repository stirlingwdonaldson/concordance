package nlp

import (
	"context"
	"time"

	"concordance/services/api-go/internal/nlpv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const (
	healthMethod          = "/concordance.nlp.v1.NLPService/Health"
	analyzeDocumentMethod = "/concordance.nlp.v1.NLPService/AnalyzeDocument"
)

type GRPCClient struct {
	conn *grpc.ClientConn
}

func NewGRPCClient(ctx context.Context, addr string) (*GRPCClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCClient{conn: conn}, nil
}

func (c *GRPCClient) Health(ctx context.Context) (*nlpv1.HealthResponse, error) {
	resp := &nlpv1.HealthResponse{}
	if err := c.invoke(ctx, healthMethod, &nlpv1.HealthRequest{}, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *GRPCClient) AnalyzeDocument(ctx context.Context, req *nlpv1.AnalyzeDocumentRequest) (*nlpv1.AnalyzeDocumentResponse, error) {
	resp := &nlpv1.AnalyzeDocumentResponse{}
	if err := c.invoke(ctx, analyzeDocumentMethod, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *GRPCClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func (c *GRPCClient) invoke(ctx context.Context, method string, req, resp proto.Message) error {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return c.conn.Invoke(callCtx, method, req, resp)
}
