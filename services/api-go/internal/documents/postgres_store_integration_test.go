package documents

import (
	"context"
	"os"
	"testing"
	"time"

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

	if err := db.RunMigrations(ctx, pool, "migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents table: %v", err)
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

	if err := db.RunMigrations(ctx, pool, "migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := pool.Exec(ctx, "delete from documents"); err != nil {
		t.Fatalf("reset documents table: %v", err)
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
	doc, err := store.Create(ctx, CreateInput{
		ProjectID:  project.ID,
		FileName:   "war-and-peace.txt",
		LocalPath:  "/tmp/war-and-peace.txt",
		SourceHash: "hash-2",
		Format:     "txt",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	store.RunPipeline(ctx, doc.ID)
	time.Sleep(200 * time.Millisecond)

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
}
