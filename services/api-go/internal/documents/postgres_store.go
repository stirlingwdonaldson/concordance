package documents

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var stages = []string{
	"ingest_parse",
	"segment_structure",
	"nlp_analyze",
	"concordance_aggregate",
	"finalize_ready",
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Document, error) {
	id := uuid.New().String()

	const query = `
insert into documents (
  id,
  project_id,
  source_path,
  source_hash,
  format,
  title,
  ingest_status,
  progress
)
values ($1, $2, $3, $4, $5, $6, 'queued', 0)
returning id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at
`

	var doc Document
	err := s.pool.QueryRow(
		ctx,
		query,
		id,
		input.ProjectID,
		input.LocalPath,
		input.SourceHash,
		input.Format,
		input.FileName,
	).Scan(
		&doc.ID,
		&doc.ProjectID,
		&doc.LocalPath,
		&doc.SourceHash,
		&doc.Format,
		&doc.FileName,
		&doc.Status,
		&doc.Progress,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		return Document{}, err
	}

	return doc, nil
}

func (s *PostgresStore) Get(ctx context.Context, documentID string) (Document, bool) {
	const query = `
select id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at
from documents
where id = $1
`

	var doc Document
	err := s.pool.QueryRow(ctx, query, documentID).Scan(
		&doc.ID,
		&doc.ProjectID,
		&doc.LocalPath,
		&doc.SourceHash,
		&doc.Format,
		&doc.FileName,
		&doc.Status,
		&doc.Progress,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, false
	}
	if err != nil {
		return Document{}, false
	}

	return doc, true
}

func (s *PostgresStore) ListByProject(ctx context.Context, projectID string) ([]Document, error) {
	const query = `
select id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at
from documents
where project_id = $1
order by created_at desc
`

	rows, err := s.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID,
			&doc.ProjectID,
			&doc.LocalPath,
			&doc.SourceHash,
			&doc.Format,
			&doc.FileName,
			&doc.Status,
			&doc.Progress,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}

		items = append(items, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) RunPipeline(ctx context.Context, documentID string) {
	for idx, stageName := range stages {
		select {
		case <-ctx.Done():
			s.markFailed(context.Background(), documentID)
			return
		default:
		}

		progress := float64(idx+1) / float64(len(stages))
		_ = s.markStage(context.Background(), documentID, stageName, progress)
		time.Sleep(150 * time.Millisecond)
	}
}

func (s *PostgresStore) markStage(ctx context.Context, documentID, stageName string, progress float64) error {
	status := stageName
	if progress >= 1 {
		status = "ready"
	}

	_, err := s.pool.Exec(ctx, `
update documents
set ingest_status = $2,
    progress = $3,
    updated_at = now()
where id = $1
`, documentID, status, progress)

	return err
}

func (s *PostgresStore) markFailed(ctx context.Context, documentID string) {
	_, _ = s.pool.Exec(ctx, `
update documents
set ingest_status = 'failed',
    updated_at = now()
where id = $1
`, documentID)
}
