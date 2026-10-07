package documents

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"concordance/services/api-go/internal/ingest"
	"concordance/services/api-go/internal/nlp"
	"concordance/services/api-go/internal/nlpv1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var stageOrder = []string{
	"ingest_parse",
	"segment_structure",
	"nlp_analyze",
	"concordance_aggregate",
	"finalize_ready",
}

var paragraphBreakPattern = regexp.MustCompile(`\n\s*\n+`)
var sentencePattern = regexp.MustCompile(`[^.!?]+[.!?]?`)

type PostgresStore struct {
	pool      *pgxpool.Pool
	events    *JobEventBroker
	nlp       nlp.Client
	extractor *ingest.Extractor
}

func NewPostgresStore(pool *pgxpool.Pool, events *JobEventBroker, nlpClient ...nlp.Client) *PostgresStore {
	return NewPostgresStoreWithTikaEndpoint(pool, events, "", nlpClient...)
}

func NewPostgresStoreWithTikaEndpoint(pool *pgxpool.Pool, events *JobEventBroker, tikaEndpoint string, nlpClient ...nlp.Client) *PostgresStore {
	var client nlp.Client
	if len(nlpClient) > 0 {
		client = nlpClient[0]
	}

	return &PostgresStore{
		pool:      pool,
		events:    events,
		nlp:       client,
		extractor: ingest.NewExtractor(tikaEndpoint),
	}
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return s.resolveDuplicate(ctx, input)
		}

		return Document{}, err
	}

	return doc, nil
}

// resolveDuplicate handles re-uploading a file that already exists in the same
// project. A document that previously failed is reset and re-queued so the
// upload acts as a retry; anything else is returned as-is without reprocessing.
func (s *PostgresStore) resolveDuplicate(ctx context.Context, input CreateInput) (Document, error) {
	var existingID string
	if err := s.pool.QueryRow(
		ctx,
		`select id from documents where project_id = $1 and source_hash = $2`,
		input.ProjectID,
		input.SourceHash,
	).Scan(&existingID); err != nil {
		return Document{}, err
	}

	existing, ok := s.Get(ctx, existingID)
	if !ok {
		return Document{}, fmt.Errorf("existing document %s not found", existingID)
	}

	if existing.Status != "failed" {
		return existing, nil
	}

	retried, found, err := s.Retry(ctx, existingID)
	if err != nil {
		return Document{}, err
	}
	if !found {
		return Document{}, fmt.Errorf("existing document %s not found", existingID)
	}

	return retried, nil
}

func (s *PostgresStore) Get(ctx context.Context, documentID string) (Document, bool) {
	const query = `
select id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at, coalesce(ingest_error, '')
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
		&doc.Error,
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
select id, project_id, source_path, source_hash, format, title, ingest_status, progress, created_at, updated_at, coalesce(ingest_error, '')
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
			&doc.Error,
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

func (s *PostgresStore) ListConcordance(ctx context.Context, documentID string, filter ConcordanceFilter) ([]ConcordanceTerm, error) {
	const query = `
select id, lemma, normalized_form, total_freq, hapax
from concordance_terms ct
where ct.document_id = $1
  and ($2 = '' or ct.lemma ilike '%' || $2 || '%')
  and ($3 = '' or exists (
    select 1
    from tokens t
    where t.document_id = ct.document_id
      and t.lemma = ct.lemma
      and coalesce(t.pos, '') = $3
  ))
  and ($4 = '' or exists (
    select 1
    from kwic_occurrences ko
    where ko.term_id = ct.id
      and ko.section_id::text = $4
  ))
order by total_freq desc, lemma asc
`

	rows, err := s.pool.Query(ctx, query, documentID, strings.TrimSpace(filter.Lemma), strings.TrimSpace(filter.POS), strings.TrimSpace(filter.Section))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ConcordanceTerm, 0)
	for rows.Next() {
		var item ConcordanceTerm
		if err := rows.Scan(&item.ID, &item.Lemma, &item.NormalizedForm, &item.TotalFreq, &item.Hapax); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) ListKWIC(ctx context.Context, documentID string, filter KWICFilter) ([]KWICOccurrence, error) {
	baseQuery := `
select ko.id, ko.term_id, ct.lemma, ko.sentence_id, ko.left_context, ko.keyword, ko.right_context, coalesce(ko.section_id::text, ''), coalesce(ko.page_ref, '')
from kwic_occurrences ko
join concordance_terms ct on ct.id = ko.term_id
where ko.document_id = $1
  and ($2 = '' or ct.lemma ilike '%' || $2 || '%')
  and ($3 = '' or coalesce(ko.page_ref, '') = $3)
  and ($4 = '' or ko.section_id::text = $4)
`

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("%s order by %s limit $5 offset $6", strings.TrimSpace(baseQuery), kwicOrderClause(filter.SortBy, filter.SortDir))

	rows, err := s.pool.Query(
		ctx,
		query,
		documentID,
		strings.TrimSpace(filter.Lemma),
		strings.TrimSpace(filter.Page),
		strings.TrimSpace(filter.Section),
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]KWICOccurrence, 0)
	for rows.Next() {
		var item KWICOccurrence
		if err := rows.Scan(
			&item.ID,
			&item.TermID,
			&item.Lemma,
			&item.SentenceID,
			&item.LeftContext,
			&item.Keyword,
			&item.RightContext,
			&item.SectionID,
			&item.PageRef,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func kwicOrderClause(sortBy, sortDir string) string {
	direction := "asc"
	if strings.EqualFold(strings.TrimSpace(sortDir), "desc") {
		direction = "desc"
	}

	column := "ko.id"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "lemma":
		column = "ct.lemma"
	case "keyword":
		column = "ko.keyword"
	}

	return fmt.Sprintf("%s %s, ko.id %s", column, direction, direction)
}

func (s *PostgresStore) CountKWIC(ctx context.Context, documentID string, filter KWICFilter) (int, error) {
	const query = `
select count(*)
from kwic_occurrences ko
join concordance_terms ct on ct.id = ko.term_id
where ko.document_id = $1
  and ($2 = '' or ct.lemma ilike '%' || $2 || '%')
  and ($3 = '' or coalesce(ko.page_ref, '') = $3)
  and ($4 = '' or ko.section_id::text = $4)
`

	var total int
	err := s.pool.QueryRow(
		ctx,
		query,
		documentID,
		strings.TrimSpace(filter.Lemma),
		strings.TrimSpace(filter.Page),
		strings.TrimSpace(filter.Section),
	).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
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

	s.publishDocumentStatus(document.ID, document.ProjectID, document.Status, document.Progress, "")

	err := s.runPipelineSafely(ctx, document)
	if err != nil {
		log.Printf("pipeline failed for document %s: %v", documentID, err)
		s.markFailed(context.Background(), documentID, err.Error())

		progress := document.Progress
		if current, ok := s.Get(context.Background(), documentID); ok {
			progress = current.Progress
		}
		s.publishDocumentStatus(document.ID, document.ProjectID, "failed", progress, err.Error())
	}
}

// runPipelineSafely converts a panic anywhere in the pipeline into an ordinary
// error so one bad document can never crash the API process.
func (s *PostgresStore) runPipelineSafely(ctx context.Context, document Document) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal error while processing document: %v", recovered)
		}
	}()

	return s.runIngestionPipeline(ctx, document)
}

func (s *PostgresStore) runIngestionPipeline(ctx context.Context, document Document) error {
	stageProgress := func(index int) float64 {
		return float64(index+1) / float64(len(stageOrder))
	}

	stageStart := time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[0], stageProgress(0)); err != nil {
		return err
	}
	s.publishStageStarted(document.ID, document.ProjectID, stageOrder[0], stageProgress(0), "loading source text")

	content, err := s.extractor.ExtractText(ctx, document.LocalPath, document.Format)
	if err != nil {
		failedAt := time.Now().UTC()
		_ = s.recordStageJob(ctx, document.ID, stageOrder[0], "failed", err.Error(), stageStart, failedAt)
		s.publishStageFailed(document.ID, document.ProjectID, stageOrder[0], stageProgress(0), err.Error(), stageStart, failedAt)
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[0], "completed", "loaded source text", stageStart, time.Now().UTC()); err != nil {
		return err
	}
	s.publishStageCompleted(document.ID, document.ProjectID, stageOrder[0], stageProgress(0), "loaded source text", stageStart, time.Now().UTC())

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[1], stageProgress(1)); err != nil {
		return err
	}
	s.publishStageStarted(document.ID, document.ProjectID, stageOrder[1], stageProgress(1), "segmenting passages and sentences")
	if err := s.persistStructure(ctx, document.ID, content); err != nil {
		failedAt := time.Now().UTC()
		_ = s.recordStageJob(ctx, document.ID, stageOrder[1], "failed", err.Error(), stageStart, failedAt)
		s.publishStageFailed(document.ID, document.ProjectID, stageOrder[1], stageProgress(1), err.Error(), stageStart, failedAt)
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[1], "completed", "stored passages and sentences", stageStart, time.Now().UTC()); err != nil {
		return err
	}
	s.publishStageCompleted(document.ID, document.ProjectID, stageOrder[1], stageProgress(1), "stored passages and sentences", stageStart, time.Now().UTC())

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[2], stageProgress(2)); err != nil {
		return err
	}
	s.publishStageStarted(document.ID, document.ProjectID, stageOrder[2], stageProgress(2), "analyzing document tokens")
	if err := s.analyzeDocument(ctx, document); err != nil {
		failedAt := time.Now().UTC()
		_ = s.recordStageJob(ctx, document.ID, stageOrder[2], "failed", err.Error(), stageStart, failedAt)
		s.publishStageFailed(document.ID, document.ProjectID, stageOrder[2], stageProgress(2), err.Error(), stageStart, failedAt)
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[2], "completed", "analyzed document tokens", stageStart, time.Now().UTC()); err != nil {
		return err
	}
	s.publishStageCompleted(document.ID, document.ProjectID, stageOrder[2], stageProgress(2), "analyzed document tokens", stageStart, time.Now().UTC())

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[3], stageProgress(3)); err != nil {
		return err
	}
	s.publishStageStarted(document.ID, document.ProjectID, stageOrder[3], stageProgress(3), "building concordance and kwic")
	if err := s.aggregateConcordance(ctx, document.ID); err != nil {
		failedAt := time.Now().UTC()
		_ = s.recordStageJob(ctx, document.ID, stageOrder[3], "failed", err.Error(), stageStart, failedAt)
		s.publishStageFailed(document.ID, document.ProjectID, stageOrder[3], stageProgress(3), err.Error(), stageStart, failedAt)
		return err
	}
	if err := s.recordStageJob(ctx, document.ID, stageOrder[3], "completed", "built concordance and kwic", stageStart, time.Now().UTC()); err != nil {
		return err
	}
	s.publishStageCompleted(document.ID, document.ProjectID, stageOrder[3], stageProgress(3), "built concordance and kwic", stageStart, time.Now().UTC())

	stageStart = time.Now().UTC()
	if err := s.markStage(ctx, document.ID, stageOrder[4], stageProgress(4)); err != nil {
		return err
	}
	s.publishStageStarted(document.ID, document.ProjectID, stageOrder[4], stageProgress(4), "finalizing document")
	if err := s.recordStageJob(ctx, document.ID, stageOrder[4], "completed", "document ready", stageStart, time.Now().UTC()); err != nil {
		return err
	}
	s.publishStageCompleted(document.ID, document.ProjectID, stageOrder[4], stageProgress(4), "document ready", stageStart, time.Now().UTC())

	_, err = s.pool.Exec(ctx, `
update documents
set ingest_status = 'ready',
    progress = 1,
    ingest_error = null,
    updated_at = now()
where id = $1
`, document.ID)
	if err == nil {
		s.publishDocumentStatus(document.ID, document.ProjectID, "ready", 1, "")
	}

	return err
}

func (s *PostgresStore) persistStructure(ctx context.Context, documentID, rawText string) error {
	normalized := strings.ReplaceAll(rawText, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	passages := splitParagraphs(normalized)

	documentUUID, err := parsePGUUID(documentID)
	if err != nil {
		return err
	}

	passageRows := make([][]any, 0, len(passages))
	sentenceRows := make([][]any, 0, len(passages))
	for idx, passage := range passages {
		passageID := uuid.New()
		sentences := splitSentences(passage)

		passageRows = append(passageRows, []any{
			pgUUID(passageID), documentUUID, int32(idx), passage.Text, passage.StartChar, passage.EndChar, int32(len(sentences)),
		})

		for sentenceIndex, sentence := range sentences {
			sentenceRows = append(sentenceRows, []any{
				pgUUID(uuid.New()), pgUUID(passageID), int32(sentenceIndex), sentence.Text, sentence.StartChar, sentence.EndChar,
			})
		}
	}

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

	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"passages"},
		[]string{"id", "document_id", "passage_index", "text", "start_char", "end_char", "sentence_count"},
		pgx.CopyFromRows(passageRows),
	); err != nil {
		return fmt.Errorf("store passages: %w", err)
	}

	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"sentences"},
		[]string{"id", "passage_id", "sentence_index", "text", "start_char", "end_char"},
		pgx.CopyFromRows(sentenceRows),
	); err != nil {
		return fmt.Errorf("store sentences: %w", err)
	}

	return tx.Commit(ctx)
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func parsePGUUID(raw string) (pgtype.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", raw, err)
	}

	return pgUUID(id), nil
}

type nlpSentenceRow struct {
	PassageID string
	ID        string
	Text      string
	StartChar int64
	EndChar   int64
}

func (s *PostgresStore) analyzeDocument(ctx context.Context, document Document) error {
	if s.nlp == nil {
		return nil
	}

	sentenceRows, err := s.loadDocumentSentences(ctx, document.ID)
	if err != nil {
		return err
	}

	batches := buildAnalyzeBatches(sentenceRows, maxBatchChars, maxBatchSentences)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from tokens where document_id = $1`, document.ID); err != nil {
		return err
	}

	for i, batch := range batches {
		response, err := s.nlp.AnalyzeDocument(ctx, &nlpv1.AnalyzeDocumentRequest{
			ProjectId:    document.ProjectID,
			DocumentId:   document.ID,
			LanguageHint: "en",
			Passages:     batch,
		})
		if err != nil {
			return fmt.Errorf("analyze batch %d of %d: %w", i+1, len(batches), err)
		}

		if err := copyTokens(ctx, tx, document.ID, response.GetTokens()); err != nil {
			return fmt.Errorf("store tokens for batch %d of %d: %w", i+1, len(batches), err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) loadDocumentSentences(ctx context.Context, documentID string) ([]nlpSentenceRow, error) {
	const query = `
select passages.id, sentences.id, sentences.text, coalesce(sentences.start_char, 0), coalesce(sentences.end_char, 0)
from passages
join sentences on sentences.passage_id = passages.id
where passages.document_id = $1
order by passages.passage_index asc, sentences.sentence_index asc
`

	rows, err := s.pool.Query(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]nlpSentenceRow, 0)
	for rows.Next() {
		var row nlpSentenceRow
		if err := rows.Scan(&row.PassageID, &row.ID, &row.Text, &row.StartChar, &row.EndChar); err != nil {
			return nil, err
		}

		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func copyTokens(ctx context.Context, tx pgx.Tx, documentID string, tokens []*nlpv1.Token) error {
	documentUUID, err := parsePGUUID(documentID)
	if err != nil {
		return err
	}

	rows := make([][]any, 0, len(tokens))
	for _, token := range tokens {
		if token.GetSentenceId() == "" {
			continue
		}

		sentenceUUID, err := parsePGUUID(token.GetSentenceId())
		if err != nil {
			return err
		}

		rows = append(rows, []any{
			documentUUID,
			sentenceUUID,
			token.GetTokenIndex(),
			token.GetSurface(),
			token.GetLemma(),
			token.GetPos(),
			token.GetIsStopword(),
			token.GetIsPunct(),
			token.GetStartChar(),
			token.GetEndChar(),
		})
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"tokens"},
		[]string{"document_id", "sentence_id", "token_index", "surface", "lemma", "pos", "is_stopword", "is_punct", "start_char", "end_char"},
		pgx.CopyFromRows(rows),
	)

	return err
}

func (s *PostgresStore) aggregateConcordance(ctx context.Context, documentID string) error {
	// Freshly bulk-inserted rows have no planner statistics, which can turn the
	// KWIC join below into a nested loop that runs for many minutes on a book.
	for _, table := range []string{"tokens", "sentences"} {
		if _, err := s.pool.Exec(ctx, "analyze "+table); err != nil {
			return err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from concordance_terms where document_id = $1`, documentID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
with lemma_pos_counts as (
  select
    document_id,
    lemma,
    coalesce(nullif(pos, ''), 'UNK') as pos,
    count(*)::int as pos_freq
  from tokens
  where document_id = $1
    and not is_punct
  group by document_id, lemma, coalesce(nullif(pos, ''), 'UNK')
), lemma_totals as (
  select
    document_id,
    lemma,
    sum(pos_freq)::int as total_freq,
    jsonb_object_agg(pos, pos_freq order by pos) as pos_distribution
  from lemma_pos_counts
  group by document_id, lemma
)
insert into concordance_terms (document_id, lemma, normalized_form, total_freq, hapax, pos_distribution, tf_stats)
select
  document_id,
  lemma,
  lower(lemma),
  total_freq,
  total_freq = 1,
  pos_distribution,
  jsonb_build_object('termFrequency', total_freq)
from lemma_totals
`, documentID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `analyze concordance_terms`); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
insert into kwic_occurrences (document_id, term_id, sentence_id, left_context, keyword, right_context)
select
  t.document_id,
  ct.id,
  t.sentence_id,
  right(substr(s.text, 1, greatest((t.start_char - coalesce(s.start_char, 0))::int, 0)), 50),
  substr(
    s.text,
    greatest((t.start_char - coalesce(s.start_char, 0))::int + 1, 1),
    greatest((t.end_char - t.start_char)::int, 1)
  ),
  left(
    substr(
      s.text,
      greatest((t.end_char - coalesce(s.start_char, 0))::int + 1, 1)
    ),
    50
  )
from tokens t
join sentences s on s.id = t.sentence_id
join concordance_terms ct
  on ct.document_id = t.document_id
 and ct.lemma = t.lemma
where t.document_id = $1
  and not t.is_punct
order by t.sentence_id, t.token_index
`, documentID); err != nil {
		return err
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

func (s *PostgresStore) publishStageStarted(documentID, projectID, stage string, progress float64, message string) {
	s.publishEvent(NewJobEvent("stage_started", documentID, projectID, map[string]any{
		"stage":    stage,
		"status":   "running",
		"progress": progress,
		"message":  message,
	}))
	s.publishDocumentStatus(documentID, projectID, stage, progress, "")
}

func (s *PostgresStore) publishStageCompleted(documentID, projectID, stage string, progress float64, message string, startedAt, finishedAt time.Time) {
	s.publishEvent(NewJobEvent("stage_completed", documentID, projectID, map[string]any{
		"stage":      stage,
		"status":     "completed",
		"progress":   progress,
		"message":    message,
		"startedAt":  startedAt,
		"finishedAt": finishedAt,
	}))
	s.publishDocumentStatus(documentID, projectID, stage, progress, "")
}

func (s *PostgresStore) publishStageFailed(documentID, projectID, stage string, progress float64, errMsg string, startedAt, finishedAt time.Time) {
	s.publishEvent(NewJobEvent("stage_failed", documentID, projectID, map[string]any{
		"stage":      stage,
		"status":     "failed",
		"progress":   progress,
		"error":      errMsg,
		"startedAt":  startedAt,
		"finishedAt": finishedAt,
	}))
}

func (s *PostgresStore) publishDocumentStatus(documentID, projectID, status string, progress float64, ingestError string) {
	payload := map[string]any{
		"status":   status,
		"progress": progress,
	}
	if ingestError != "" {
		payload["ingestError"] = ingestError
	}

	s.publishEvent(NewJobEvent("document_status", documentID, projectID, payload))
}

func (s *PostgresStore) publishEvent(event JobEvent) {
	if s.events == nil {
		return
	}

	s.events.Publish(event)
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
