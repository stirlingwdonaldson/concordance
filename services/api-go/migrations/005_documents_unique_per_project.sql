-- Allow the same file to be uploaded into different projects. Duplicate
-- uploads within one project are detected by (project_id, source_hash) and
-- resolved to the existing document instead of failing.
alter table documents drop constraint if exists documents_source_hash_key;

create unique index if not exists idx_documents_project_source_hash
  on documents(project_id, source_hash);
