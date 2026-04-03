package documents

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"concordance/services/api-go/internal/db"
	"concordance/services/api-go/internal/projects"
)

func TestPostgresStoreCreateGetAndList(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from pipeline_jobs"); err != nil {
		t.Fatalf("reset pipeline jobs table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from sentences"); err != nil {
		t.Fatalf("reset sentences table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from passages"); err != nil {
		t.Fatalf("reset passages table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from projects"); err != nil {
		t.Fatalf("reset projects table: %v", err)
	}

	projectStore := projects.NewPostgresStore(pool)
	project, err := projectStore.Create(ctx, projects.CreateProjectInput{Name: "Integration Project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	store := NewPostgresStore(pool)
	created, err := store.Create(ctx, CreateInput{
		ProjectID:  project.ID,
		FileName:   "sample.txt",
		LocalPath:  "/tmp/sample.txt",
		SourceHash: "hash-1",
		Format:     "txt",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	loaded, ok := store.Get(ctx, created.ID)
	if !ok {
		t.Fatalf("expected created document to load")
	}

	if loaded.Status != "queued" {
		t.Fatalf("unexpected initial status: got=%s", loaded.Status)
	}

	items, err := store.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected document count: got=%d", len(items))
	}
}

func TestPostgresStoreRunPipeline(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from pipeline_jobs"); err != nil {
		t.Fatalf("reset pipeline jobs table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from sentences"); err != nil {
		t.Fatalf("reset sentences table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from passages"); err != nil {
		t.Fatalf("reset passages table: %v", err)
	}
	if _, err := pool.Exec(ctx, "delete from projects"); err != nil {
		t.Fatalf("reset projects table: %v", err)
	}

	projectStore := projects.NewPostgresStore(pool)
	project, err := projectStore.Create(ctx, projects.CreateProjectInput{Name: "Pipeline Project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	store := NewPostgresStore(pool)
	tmpDir := t.TempDir()
	textPath := filepath.Join(tmpDir, "war-and-peace.txt")
	if err := os.WriteFile(textPath, []byte("Call me Ishmael. Some years ago.\n\nThis is a second paragraph."), 0o644); err != nil {
		t.Fatalf("write source text: %v", err)
	}

	doc, err := store.Create(ctx, CreateInput{
		ProjectID:  project.ID,
		FileName:   "war-and-peace.txt",
		LocalPath:  textPath,
		SourceHash: "hash-2",
		Format:     "txt",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	store.RunPipeline(ctx, doc.ID)

	updated, ok := store.Get(ctx, doc.ID)
	if !ok {
		t.Fatalf("expected document to load after pipeline")
	}

	if updated.Status != "ready" {
		t.Fatalf("unexpected final status: got=%s", updated.Status)
	}

	if updated.Progress != 1 {
		t.Fatalf("unexpected final progress: got=%v", updated.Progress)
	}

	passages, err := store.ListPassages(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list passages: %v", err)
	}
	if len(passages) != 2 {
		t.Fatalf("unexpected passages count: got=%d", len(passages))
	}

	sentences, err := store.ListSentences(ctx, doc.ID, "")
	if err != nil {
		t.Fatalf("list sentences: %v", err)
	}
	if len(sentences) < 3 {
		t.Fatalf("expected at least 3 sentences, got=%d", len(sentences))
	}

	jobs, err := store.ListPipelineJobs(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list pipeline jobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("unexpected pipeline job count: got=%d", len(jobs))
	}
	if jobs[0].Stage != "ingest_parse" {
		t.Fatalf("unexpected first stage: got=%s", jobs[0].Stage)
	}

	retried, ok, err := store.Retry(ctx, doc.ID)
	if err != nil {
		t.Fatalf("retry document: %v", err)
	}
	if !ok {
		t.Fatalf("expected retry to find document")
	}
	if retried.Status != "queued" {
		t.Fatalf("unexpected retry status: got=%s", retried.Status)
	}

	retryJobs, err := store.ListPipelineJobs(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list pipeline jobs after retry: %v", err)
	}
	if len(retryJobs) != 0 {
		t.Fatalf("expected retry to clear previous jobs, got=%d", len(retryJobs))
	}
}
