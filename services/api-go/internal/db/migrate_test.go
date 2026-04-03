package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrationsSorted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "002_b.sql"), []byte("select 2;"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_a.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("unexpected migration count: got=%d", len(migrations))
	}

	if migrations[0].Name != "001_a.sql" {
		t.Fatalf("unexpected first migration: got=%s", migrations[0].Name)
	}

	if migrations[1].Name != "002_b.sql" {
		t.Fatalf("unexpected second migration: got=%s", migrations[1].Name)
	}
}

func TestLoadMigrationsRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_empty.sql"), []byte("   \n"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	if _, err := LoadMigrations(dir); err == nil {
		t.Fatalf("expected empty migration file error")
	}
}
