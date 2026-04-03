# Concordance Desktop App — Phase 1 Implementation Blueprint

## Goal

Deliver an offline macOS desktop MVP that ingests texts, builds concordance/KWIC, performs baseline NER, and supports semantic search within a project.

## Architecture (MVP Lock-In)

- Desktop shell: Tauri
- Frontend: Next.js (SPA-style in desktop shell)
- Backend/orchestrator: Go
- NLP sidecar: Python gRPC service
- Storage/search: PostgreSQL + pgvector + Postgres FTS

## Scope Discipline

Deferred to Phase 2+:

- Advanced coreference adjudication UX
- Cross-text entity linking workflows
- Etymology drift + historical contextualization UX
- Full translation UX and alignment tools

---

## 1) Contracts First (Go <-> Python gRPC)

Create `packages/proto/nlp.proto` with these services/messages:

- `NLPService.AnalyzeDocument`
  - Input: document ID + passages/sentences + language hints + stopword config
  - Output: tokens (surface, lemma, POS, offsets, stopword), sentence language tags, baseline entities/mentions
- `NLPService.EmbedPassages`
  - Input: passage IDs + text
  - Output: embedding vectors + model metadata (`name`, `dim`)
- `NLPService.Health`
  - Input: empty
  - Output: model load status, versions, readiness

MVP API decisions:

- Batch operations (no single-row RPC loops)
- Embeddings returned as `float32`
- Include `pipeline_version` in responses

Embedding model lock for MVP:

- `all-MiniLM-L6-v2` (384 dimensions)

---

## 2) Database Migration Plan

Create migrations in this order:

1. `001_core_projects_documents.sql`
   - `projects`, `documents`, `document_sections`, `passages`, `sentences`
2. `002_nlp_tokens_concordance.sql`
   - `tokens`, `concordance_terms`, `kwic_occurrences`, `collocations`, `vocab_metrics`
3. `003_entities_graph.sql`
   - `entities`, `entity_aliases`, `entity_mentions`, `entity_edges`, `entity_timeline`
4. `004_search_embeddings_fts.sql`
   - `passage_embeddings` (vector), `fts_passages`, `cross_text_links`, `cross_entity_links`
5. `005_user_annotations_jobs_exports.sql`
   - `annotations`, `pipeline_jobs`, `exports`
6. `006_indexes_perf.sql`
   - ivfflat, gin, and composite indexes for query hot paths

MVP index priorities:

- `tokens(document_id, lemma)`
- `kwic_occurrences(term_id)`
- `entity_mentions(document_id, surface)`
- `passage_embeddings` ivfflat cosine index
- `fts_passages` gin index

---

## 3) Go API Surface (Phase 1)

- `POST /api/projects`
- `GET /api/projects`
- `POST /api/projects/{projectId}/documents/upload`
- `GET /api/projects/{projectId}/documents`
- `GET /api/documents/{documentId}/status`
- `GET /api/documents/{documentId}/concordance?lemma=&pos=&section=`
- `GET /api/documents/{documentId}/kwic?lemma=&page=&section=&limit=&offset=`
- `GET /api/documents/{documentId}/entities?type=`
- `GET /api/documents/{documentId}/search/lexical?q=`
- `GET /api/documents/{documentId}/search/semantic?q=`
- `POST /api/annotations`
- `GET /api/projects/{projectId}/annotations`
- `POST /api/exports/concordance` (CSV/JSON)

Realtime:

- `GET /ws/jobs` for pipeline progress and stage events

---

## 4) Processing Pipeline (Orchestrated Jobs)

Persist stage transitions in `pipeline_jobs`:

1. `ingest_parse`
2. `segment_structure`
3. `nlp_analyze` (Python gRPC)
4. `concordance_aggregate`
5. `embed_passages` (Python gRPC)
6. `entity_materialize`
7. `fts_refresh`
8. `finalize_ready`

Per-stage rules:

- Persist progress percentage and timestamps
- Capture stage errors with structured logs
- Make stage handlers idempotent (safe retry)

---

## 5) Frontend MVP Screens

- Project list + create project
- Document upload + processing status
- Concordance table (lemma/freq/POS/hapax filters)
- KWIC explorer (sortable, filterable, paginated)
- Entity list + mention context panel
- Semantic search panel (query + top-k passages)
- Annotation drawer

Frontend state/data strategy:

- React Query for API-backed state
- Zustand for transient UI state
- Virtualized tables for KWIC/concordance scaling

---

## 6) Sidecar and Lifecycle Strategy

Tauri shell responsibilities:

- Start Go API sidecar on app launch
- Expose app menu shortcuts and native window controls

Go API responsibilities:

- Start/stop Python NLP sidecar
- Run startup health checks:
  - Python gRPC health
  - Postgres connection and extension checks (`pgvector`)

Graceful shutdown order:

1. Stop accepting new jobs
2. Drain active jobs
3. Stop Python sidecar
4. Stop Go API

---

## 7) Milestone Plan (Suggested 10 Weeks)

- Week 1-2: monorepo scaffold, DB bootstrap, core migrations, project/document CRUD
- Week 3-4: ingestion/parsing, section/sentence persistence, WebSocket job updates
- Week 5-6: NLP token/POS/lemma pipeline, concordance + KWIC APIs and UI
- Week 7: embeddings + semantic search
- Week 8: baseline NER mentions/entities + UI explorer
- Week 9: annotations + CSV/JSON export + packaging hardening
- Week 10: performance tuning and QA on large texts

---

## 8) Phase 1 Acceptance Criteria

- Fully offline first run with no mandatory cloud dependency
- 1M+ word corpus can ingest and become browseable without crashes
- Concordance/KWIC query latency stays interactive (typical <1s filtered, heavy <3s)
- Semantic search returns consistent top-k similar passages
- Entity mentions render with context and source references
- CSV/JSON exports reflect active filters

---

## 9) First Sprint Backlog

1. Scaffold monorepo folders and service entry points
2. Add proto contract + codegen workflow (Go/Python stubs)
3. Implement migrations `001` and `002`
4. Build Go upload/parse skeleton + queued jobs
5. Implement WebSocket job progress stream
6. Ship first UI for upload/status/concordance placeholder
