package projects

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateProjectInput) (Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Project{}, ErrInvalidName
	}

	id := uuid.New().String()
	description := strings.TrimSpace(input.Description)

	const query = `
insert into projects (id, name, description)
values ($1, $2, $3)
returning id, name, coalesce(description, ''), created_at, updated_at
`

	var project Project
	err := s.pool.QueryRow(ctx, query, id, name, description).Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return Project{}, err
	}

	return project, nil
}

func (s *PostgresStore) List(ctx context.Context) ([]Project, error) {
	const query = `
select id, name, coalesce(description, ''), created_at, updated_at
from projects
order by created_at desc
`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Name, &project.Description, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}
