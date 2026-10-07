-- Foreign keys with "on delete cascade" need an index on the referencing column,
-- otherwise deleting sentences (document retry, project/document delete) scans
-- the whole table once per deleted row and takes minutes on a large text.
create index if not exists idx_tokens_sentence_id on tokens(sentence_id);
create index if not exists idx_kwic_sentence_id on kwic_occurrences(sentence_id);

-- KWIC listing always filters by document, then joins to concordance terms.
create index if not exists idx_kwic_document_term on kwic_occurrences(document_id, term_id);
