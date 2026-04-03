create table if not exists pipeline_jobs (
  id uuid primary key,
  document_id uuid not null references documents(id) on delete cascade,
  stage text not null,
  status text not null,
  message text,
  started_at timestamptz not null,
  finished_at timestamptz not null
);

create index if not exists idx_pipeline_jobs_document_id on pipeline_jobs(document_id);
create index if not exists idx_pipeline_jobs_stage on pipeline_jobs(stage);
