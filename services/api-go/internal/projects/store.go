package projects

import "context"

type Store interface {
	Create(ctx context.Context, input CreateProjectInput) (Project, error)
	List(ctx context.Context) ([]Project, error)
	Update(ctx context.Context, id string, input UpdateProjectInput) (Project, bool, error)
	Delete(ctx context.Context, id string) (bool, error)
}
