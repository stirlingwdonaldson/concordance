package projects

import (
	"context"
	"os"
	"testing"

	"concordance/services/api-go/internal/db"
)

func TestPostgresStoreCreateAndList(t *testing.T) {
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

	if _, err := pool.Exec(ctx, "delete from projects"); err != nil {
		t.Fatalf("reset projects table: %v", err)
	}

	store := NewPostgresStore(pool)
	created, err := store.Create(ctx, CreateProjectInput{Name: "Austen", Description: "Jane Austen corpus"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if created.ID == "" {
		t.Fatalf("expected generated project id")
	}

	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("unexpected project count: got=%d", len(items))
	}

	if items[0].Name != "Austen" {
		t.Fatalf("unexpected project name: got=%s", items[0].Name)
	}
}
