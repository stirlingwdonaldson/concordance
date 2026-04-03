package documents

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var stageOrder = []string{
	"ingest_parse",
	"segment_structure",
	"finalize_ready",
}

var paragraphBreakPattern = regexp.MustCompile(`\n\s*\n+`)
var sentencePattern = regexp.MustCompile(`[^.!?]+[.!?]?`)

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

func (s *PostgresStore) ListPassages(ctx context.Context, documentID string) ([]Passage, error) {
	const query = `
select id, passage_index, text, coalesce(start_char, 0), coalesce(end_char, 0), sentence_count
from passages
where document_id = $1
order by passage_index asc
`

	rows, err := s.pool.Query(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Passage, 0)
	for rows.Next() {
		var p Passage
		if err := rows.Scan(&p.ID, &p.PassageIndex, &p.Text, &p.StartChar, &p.EndChar, &p.SentenceCount); err != nil {
			return nil, err
		}
		items = append(items, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) ListSentences(ctx context.Context, documentID, passageID string) ([]Sentence, error) {
	query := `
select sentences.id, sentences.passage_id, sentences.sentence_index, sentences.text, coalesce(sentences.start_char, 0), coalesce(sentences.end_char, 0)
from sentences
join passages on passages.id = sentences.passage_id
where passages.document_id = $1
`
	args := []any{documentID}
	if passageID != "" {
		query += " and sentences.passage_id = $2"
		args = append(args, passageID)
	}
	query += " order by passages.passage_index asc, sentences.sentence_index asc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Sentence, 0)
	for rows.Next() {
		var sentence Sentence
		if err := rows.Scan(
			&sentence.ID,
			&sentence.PassageID,
			&sentence.SentenceIndex,
			&sentence.Text,
			&sentence.StartChar,
			&sentence.EndChar,
		); err != nil {
			return nil, err
		}

		items = append(items, sentence)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) ListPipelineJobs(ctx context.Context, documentID string) ([]PipelineJob, error) {
	const query = `
select id, stage, status, coalesce(message, ''), started_at, finished_at
from pipeline_jobs
where document_id = $1
order by started_at asc
`

	rows, err := s.pool.Query(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]PipelineJob, 0)
	for rows.Next() {
		var job PipelineJob
		if err := rows.Scan(
			&job.ID,
			&job.Stage,
			&job.Status,
			&job.Message,
			&job.StartedAt,
			&job.FinishedAt,
		); err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *PostgresStore) Retry(ctx context.Context, documentID string) (Document, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Document{}, false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
delete from pipeline_jobs where document_id = $1
`, documentID); err != nil {
		return Document{}, false, err
	}

	if _, err := tx.Exec(ctx, `
delete from sentences
where passage_id in (select id from passages where document_id = $1)
`, documentID); err != nil {
		return Document{}, false, err
	}

	if _, err := tx.Exec(ctx, `delete from passages where document_id = $1`, documentID); err != nil {
		return Document{}, false, err
	}

	const updateQuery = `
update documents
set ingest_status = 'queued',
    progress = 0,
    ingest_error = null,
    updated_at = now()
where id = $1
returning id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at
`

	var doc Document
	err = tx.QueryRow(ctx, updateQuery, documentID).Scan(
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
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Document{}, false, err
	}

	return doc, true, nil
}

func (s *PostgresStore) RunPipeline(ctx context.Context, documentID string) {
	document, ok := s.Get(ctx, documentID)
	if !ok {
		return
	}

	if err := s.runIngestionPipeline(ctx, document); err != nil {
		s.markFailed(context.Background(), documentID, err.Error())
	}
}

func (s *PostgresStore) runIngestionPipeline(ctx context.Context, document Document) error {
	if document.Format != "txt" {
		return errors.New("only txt ingestion is supported in this phase")
	}

	stageStart := time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[0], 0.34); err != nil {
		return err
	}

	content, err := os.ReadFile(document.LocalPath)
	if err != nil {
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[0], "completed", "loaded source text", stageStart, time.Now().UTC()); err != nil {
		return err
	}

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[1], 0.67); err != nil {
		return err
	}
	if err := s.persistStructure(ctx, document.ID, string(content)); err != nil {
		_ = s.recordStageJob(ctx, document.ID, stageOrder[1], "failed", err.Error(), stageStart, time.Now().UTC())
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[1], "completed", "stored passages and sentences", stageStart, time.Now().UTC()); err != nil {
		return err
	}

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[2], 1); err != nil {
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[2], "completed", "document ready", stageStart, time.Now().UTC()); err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
update documents
set ingest_status = 'ready',
    progress = 1,
    ingest_error = null,
    updated_at = now()
where id = $1
`, document.ID)
	return err
}

func (s *PostgresStore) persistStructure(ctx context.Context, documentID, rawText string) error {
	normalized := strings.ReplaceAll(rawText, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	passages := splitParagraphs(normalized)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
delete from sentences
where passage_id in (select id from passages where document_id = $1)
`, documentID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `delete from passages where document_id = $1`, documentID); err != nil {
		return err
	}

	for idx, passage := range passages {
		passageID := uuid.New().String()
		sentences := splitSentences(passage)

		if _, err := tx.Exec(ctx, `
insert into passages (id, document_id, passage_index, text, start_char, end_char, sentence_count)
values ($1, $2, $3, $4, $5, $6, $7)
`, passageID, documentID, idx, passage.Text, passage.StartChar, passage.EndChar, len(sentences)); err != nil {
			return err
		}

		for sentenceIndex, sentence := range sentences {
			if _, err := tx.Exec(ctx, `
insert into sentences (id, passage_id, sentence_index, text, start_char, end_char)
values ($1, $2, $3, $4, $5, $6)
`, uuid.New().String(), passageID, sentenceIndex, sentence.Text, sentence.StartChar, sentence.EndChar); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) markStage(ctx context.Context, documentID, stageName string, progress float64) error {
	_, err := s.pool.Exec(ctx, `
update documents
set ingest_status = $2,
    progress = $3,
    updated_at = now()
where id = $1
`, documentID, stageName, progress)

	return err
}

func (s *PostgresStore) markFailed(ctx context.Context, documentID, message string) {
	_, _ = s.pool.Exec(ctx, `
update documents
set ingest_status = 'failed',
    ingest_error = $2,
    updated_at = now()
where id = $1
`, documentID, message)
}

func (s *PostgresStore) recordStageJob(ctx context.Context, documentID, stage, status, message string, startedAt, finishedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
insert into pipeline_jobs (id, document_id, stage, status, message, started_at, finished_at)
values ($1, $2, $3, $4, $5, $6, $7)
`, uuid.New().String(), documentID, stage, status, message, startedAt, finishedAt)

	return err
}

type passageSegment struct {
	Text      string
	StartChar int64
	EndChar   int64
}

type sentenceSegment struct {
	Text      string
	StartChar int64
	EndChar   int64
}

func splitParagraphs(text string) []passageSegment {
	ranges := paragraphBreakPattern.FindAllStringIndex(text, -1)
	segments := make([]passageSegment, 0)
	start := 0

	for _, boundary := range ranges {
		segment := createPassageSegment(text, start, boundary[0])
		if segment.Text != "" {
			segments = append(segments, segment)
		}
		start = boundary[1]
	}

	segment := createPassageSegment(text, start, len(text))
	if segment.Text != "" {
		segments = append(segments, segment)
	}

	if len(segments) == 0 && strings.TrimSpace(text) != "" {
		segments = append(segments, createPassageSegment(text, 0, len(text)))
	}

	return segments
}

func createPassageSegment(text string, start, end int) passageSegment {
	if start >= end {
		return passageSegment{}
	}

	raw := text[start:end]
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return passageSegment{}
	}

	leading := len(raw) - len(strings.TrimLeft(raw, " \n\t"))
	trailing := len(raw) - len(strings.TrimRight(raw, " \n\t"))

	startChar := int64(start + leading)
	endChar := int64(end - trailing)

	return passageSegment{Text: trimmed, StartChar: startChar, EndChar: endChar}
}

func splitSentences(passage passageSegment) []sentenceSegment {
	matches := sentencePattern.FindAllStringIndex(passage.Text, -1)
	if len(matches) == 0 {
		return []sentenceSegment{{
			Text:      passage.Text,
			StartChar: passage.StartChar,
			EndChar:   passage.EndChar,
		}}
	}

	sentences := make([]sentenceSegment, 0, len(matches))
	for _, bounds := range matches {
		raw := passage.Text[bounds[0]:bounds[1]]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		leading := len(raw) - len(strings.TrimLeft(raw, " \n\t"))
		trailing := len(raw) - len(strings.TrimRight(raw, " \n\t"))
		startChar := passage.StartChar + int64(bounds[0]+leading)
		endChar := passage.StartChar + int64(bounds[1]-trailing)

		sentences = append(sentences, sentenceSegment{Text: trimmed, StartChar: startChar, EndChar: endChar})
	}

	if len(sentences) == 0 {
		return []sentenceSegment{{
			Text:      passage.Text,
			StartChar: passage.StartChar,
			EndChar:   passage.EndChar,
		}}
	}

	return sentences
}
