create extension if not exists vector;

create table if not exists projects (
  id uuid primary key,
  name text not null,
  description text,
  settings jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists documents (
  id uuid primary key,
  project_id uuid not null references projects(id) on delete cascade,
  source_path text not null,
  source_hash text not null,
  format text not null,
  title text,
  author text,
  publication_year int,
  language_primary text,
  region text,
  ingest_status text not null default 'queued',
  ingest_error text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (source_hash)
);

create table if not exists document_sections (
  id uuid primary key,
  document_id uuid not null references documents(id) on delete cascade,
  parent_section_id uuid references document_sections(id) on delete set null,
  section_type text not null,
  ordinal int not null,
  label text,
  start_char bigint,
  end_char bigint
);

create table if not exists passages (
  id uuid primary key,
  document_id uuid not null references documents(id) on delete cascade,
  section_id uuid references document_sections(id) on delete set null,
  passage_index int not null,
  text text not null,
  language text,
  start_char bigint,
  end_char bigint,
  page_ref text,
  sentence_count int not null default 0
);

create table if not exists sentences (
  id uuid primary key,
  passage_id uuid not null references passages(id) on delete cascade,
  sentence_index int not null,
  text text not null,
  language text,
  start_char bigint,
  end_char bigint
);

create index if not exists idx_documents_project_id on documents(project_id);
create index if not exists idx_passages_document_id on passages(document_id);
create index if not exists idx_sentences_passage_id on sentences(passage_id);
