-- Each KWIC line remembers the part of speech of its keyword so the UI can
-- filter "learning" the noun from "learning" the verb. Files processed before
-- this migration have NULL here until they are reprocessed.
alter table kwic_occurrences add column if not exists pos text;
create index if not exists idx_kwic_document_pos on kwic_occurrences(document_id, pos);
