package documents

import (
	"context"
	"strings"
)

// Keyness compares word use in one document against a reference document with
// the log-likelihood (G2) statistic, the standard measure in corpus
// linguistics. A higher score means the difference is less likely to be chance:
// above 6.63 is significant at p < 0.01 and above 15.13 at p < 0.0001.
func (s *PostgresStore) Keyness(ctx context.Context, documentID string, filter KeynessFilter) ([]KeyTerm, int, error) {
	const query = `
with a as (
  select lemma, total_freq::float8 as f, pos_distribution as pd from concordance_terms where document_id = $1
), b as (
  select lemma, total_freq::float8 as f, pos_distribution as pd from concordance_terms where document_id = $2
), n as (
  select (select coalesce(sum(f), 0) from a) as na, (select coalesce(sum(f), 0) from b) as nb
), j as (
  select coalesce(a.lemma, b.lemma) as lemma, coalesce(a.f, 0) as fa, coalesce(b.f, 0) as fb, a.pd as pda, b.pd as pdb
  from a full join b on a.lemma = b.lemma
), s as (
  select j.lemma, j.fa, j.fb, n.na, n.nb,
         (j.fa + j.fb) * n.na / (n.na + n.nb) as e1,
         (j.fa + j.fb) * n.nb / (n.na + n.nb) as e2
  from j, n
  where n.na > 0 and n.nb > 0
    and j.fa + j.fb >= $3
    and ($4 = '' or j.lemma ilike '%' || $4 || '%' escape '\')
    and ($5 = '' or jsonb_exists(coalesce(j.pda, '{}'::jsonb), $5) or jsonb_exists(coalesce(j.pdb, '{}'::jsonb), $5))
), g as (
  select lemma, fa, fb, na, nb,
         2 * ((case when fa > 0 then fa * ln(fa / e1) else 0 end) + (case when fb > 0 then fb * ln(fb / e2) else 0 end)) as score,
         case when fa / na >= fb / nb then 'a' else 'b' end as side
  from s
)
select lemma, fa::int, fb::int, fa / na * 1000000, fb / nb * 1000000, score, side, count(*) over ()
from g
where ($6 = '' or side = $6)
order by score desc, lemma asc
limit $7 offset $8
`
	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 500 {
		limit = 500
	}
	offset := max(filter.Offset, 0)
	minFreq := max(filter.MinFreq, 1)

	run := func(limit, offset int) ([]KeyTerm, int, error) {
		rows, err := s.pool.Query(ctx, query,
			documentID, filter.Against, float64(minFreq),
			escapeLike(strings.TrimSpace(filter.Lemma)), strings.TrimSpace(filter.POS),
			strings.TrimSpace(filter.Side), limit, offset,
		)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		items := make([]KeyTerm, 0)
		total := 0
		for rows.Next() {
			var item KeyTerm
			if err := rows.Scan(&item.Lemma, &item.FreqA, &item.FreqB, &item.PerMillionA, &item.PerMillionB, &item.Score, &item.Side, &total); err != nil {
				return nil, 0, err
			}
			items = append(items, item)
		}

		return items, total, rows.Err()
	}

	items, total, err := run(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 && offset > 0 {
		// Past the last page: ask for the first row only to learn the real total.
		_, total, err = run(1, 0)
	}

	return items, total, err
}
