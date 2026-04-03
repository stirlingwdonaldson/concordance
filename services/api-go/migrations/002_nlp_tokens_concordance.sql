create table if not exists tokens (
  id bigserial primary key,
  document_id uuid not null references documents(id) on delete cascade,
  sentence_id uuid not null references sentences(id) on delete cascade,
  token_index int not null,
  surface text not null,
  lemma text not null,
  pos text,
  morph jsonb,
  is_stopword boolean not null default false,
  is_punct boolean not null default false,
  start_char bigint,
  end_char bigint
);

create table if not exists concordance_terms (
  id bigserial primary key,
  document_id uuid not null references documents(id) on delete cascade,
  lemma text not null,
  normalized_form text not null,
  total_freq int not null,
  hapax boolean not null default false,
  pos_distribution jsonb not null default '{}'::jsonb,
  tf_stats jsonb not null default '{}'::jsonb,
  unique (document_id, lemma)
);

create table if not exists kwic_occurrences (
  id bigserial primary key,
  document_id uuid not null references documents(id) on delete cascade,
  term_id bigint not null references concordance_terms(id) on delete cascade,
  sentence_id uuid not null references sentences(id) on delete cascade,
  left_context text not null,
  keyword text not null,
  right_context text not null,
  section_id uuid references document_sections(id) on delete set null,
  page_ref text
);

create table if not exists collocations (
  id bigserial primary key,
  document_id uuid not null references documents(id) on delete cascade,
  lemma_a text not null,
  lemma_b text not null,
  window_size int not null,
  score_pmi double precision,
  score_llr double precision,
  count int not null default 0
);

create table if not exists vocab_metrics (
  document_id uuid primary key references documents(id) on delete cascade,
  token_count bigint not null,
  type_count bigint not null,
  type_token_ratio double precision not null,
  moving_ttr jsonb not null default '{}'::jsonb
);

create index if not exists idx_tokens_document_lemma on tokens(document_id, lemma);
create index if not exists idx_tokens_document_pos on tokens(document_id, pos);
create index if not exists idx_kwic_term_id on kwic_occurrences(term_id);
create index if not exists idx_collocations_document_score on collocations(document_id, score_llr desc);
