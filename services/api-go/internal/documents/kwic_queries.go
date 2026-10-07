package documents

import (
	"context"
	"fmt"
	"strings"
)

// escapeLike makes user text match literally inside an ILIKE pattern.
func escapeLike(raw string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	return replacer.Replace(raw)
}

type kwicQuery struct {
	where string
	args  []any
}

// buildKWICWhere builds the shared filter. scope is a SQL condition using $1,
// for example "ko.document_id = $1" or "d.project_id = $1".
func buildKWICWhere(scope, scopeID string, filter KWICFilter) kwicQuery {
	query := kwicQuery{where: scope, args: []any{scopeID}}
	add := func(condition string, value any) {
		query.args = append(query.args, value)
		query.where += " and " + strings.ReplaceAll(condition, "$N", fmt.Sprintf("$%d", len(query.args)))
	}

	if lemma := strings.TrimSpace(filter.Lemma); lemma != "" {
		// Whole words by default, matching either the dictionary form or the
		// printed form ("ran" finds run). A * widens it: learn* finds learning.
		if strings.Contains(lemma, "*") {
			pattern := strings.ReplaceAll(escapeLike(lemma), "*", "%")
			add(`(ct.lemma ilike $N escape '\' or ko.keyword ilike $N escape '\')`, pattern)
		} else {
			add(`(ct.lemma = lower($N) or lower(ko.keyword) = lower($N))`, lemma)
		}
	}
	if page := strings.TrimSpace(filter.Page); page != "" {
		add(`coalesce(ko.page_ref, '') = $N`, page)
	}
	if section := strings.TrimSpace(filter.Section); section != "" {
		add(`ko.section_id::text = $N`, section)
	}
	if pos := strings.TrimSpace(filter.POS); pos != "" {
		add(`ko.pos = $N`, pos)
	}

	return query
}

func kwicOrderClause(sortBy, sortDir string, acrossDocuments bool) string {
	direction := "asc"
	if strings.EqualFold(strings.TrimSpace(sortDir), "desc") {
		direction = "desc"
	}

	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "lemma":
		return fmt.Sprintf("ct.lemma %s, ko.id %s", direction, direction)
	case "keyword":
		return fmt.Sprintf("ko.keyword %s, ko.id %s", direction, direction)
	}
	if acrossDocuments {
		return fmt.Sprintf("d.title %s, d.id %s, ko.id %s", direction, direction, direction)
	}

	return fmt.Sprintf("ko.id %s", direction)
}

const kwicFrom = `
from kwic_occurrences ko
join concordance_terms ct on ct.id = ko.term_id
join documents d on d.id = ko.document_id
`

func (s *PostgresStore) listKWIC(ctx context.Context, scope, scopeID string, filter KWICFilter, across bool) ([]KWICOccurrence, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := max(filter.Offset, 0)

	built := buildKWICWhere(scope, scopeID, filter)
	built.args = append(built.args, limit, offset)
	query := fmt.Sprintf(`
select ko.id, ko.term_id, ct.lemma, ko.sentence_id, ko.left_context, ko.keyword, ko.right_context,
       coalesce(ko.section_id::text, ''), coalesce(ko.page_ref, ''), coalesce(ko.pos, ''),
       ko.document_id::text, d.title
%s
where %s
order by %s
limit $%d offset $%d`,
		kwicFrom, built.where, kwicOrderClause(filter.SortBy, filter.SortDir, across),
		len(built.args)-1, len(built.args))

	rows, err := s.pool.Query(ctx, query, built.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]KWICOccurrence, 0)
	for rows.Next() {
		var item KWICOccurrence
		if err := rows.Scan(
			&item.ID, &item.TermID, &item.Lemma, &item.SentenceID,
			&item.LeftContext, &item.Keyword, &item.RightContext,
			&item.SectionID, &item.PageRef, &item.POS,
			&item.DocumentID, &item.DocumentName,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *PostgresStore) countKWIC(ctx context.Context, scope, scopeID string, filter KWICFilter) (int, error) {
	built := buildKWICWhere(scope, scopeID, filter)
	var total int
	err := s.pool.QueryRow(ctx, "select count(*) "+kwicFrom+" where "+built.where, built.args...).Scan(&total)

	return total, err
}

func (s *PostgresStore) ListKWIC(ctx context.Context, documentID string, filter KWICFilter) ([]KWICOccurrence, error) {
	return s.listKWIC(ctx, "ko.document_id = $1", documentID, filter, false)
}

func (s *PostgresStore) CountKWIC(ctx context.Context, documentID string, filter KWICFilter) (int, error) {
	return s.countKWIC(ctx, "ko.document_id = $1", documentID, filter)
}

func (s *PostgresStore) ListProjectKWIC(ctx context.Context, projectID string, filter KWICFilter) ([]KWICOccurrence, error) {
	return s.listKWIC(ctx, "d.project_id = $1", projectID, filter, true)
}

func (s *PostgresStore) CountProjectKWIC(ctx context.Context, projectID string, filter KWICFilter) (int, error) {
	return s.countKWIC(ctx, "d.project_id = $1", projectID, filter)
}

// PartsOfSpeech counts tokens per tag, derived from the per-word tag counts.
func (s *PostgresStore) PartsOfSpeech(ctx context.Context, documentID string) ([]POSCount, error) {
	rows, err := s.pool.Query(ctx, `
select pos.key, sum(pos.value::int)::int as n
from concordance_terms ct, jsonb_each_text(ct.pos_distribution) as pos
where ct.document_id = $1
group by pos.key
order by n desc, pos.key asc
`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]POSCount, 0)
	for rows.Next() {
		var item POSCount
		if err := rows.Scan(&item.POS, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
