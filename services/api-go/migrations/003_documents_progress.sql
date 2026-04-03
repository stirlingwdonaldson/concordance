alter table documents
add column if not exists progress double precision not null default 0;
