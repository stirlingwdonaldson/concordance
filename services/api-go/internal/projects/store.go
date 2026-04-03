package projects

import "context"

type Store interface {
	Create(ctx context.Context, input CreateProjectInput) (Project, error)
	List(ctx context.Context) ([]Project, error)
}
