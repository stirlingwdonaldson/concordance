package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migration struct {
	Name string
	SQL  string
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	migrations, err := LoadMigrations(dir)
	if err != nil {
		return err
	}

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := isApplied(ctx, pool, migration.Name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := runSQLScript(ctx, pool, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}

		if _, err := pool.Exec(ctx, "insert into schema_migrations (name) values ($1)", migration.Name); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	migrations := make([]Migration, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		sql := strings.TrimSpace(string(data))
		if sql == "" {
			return nil, fmt.Errorf("migration %s is empty", file)
		}

		migrations = append(migrations, Migration{Name: file, SQL: sql})
	}

	if len(migrations) == 0 {
		return nil, errors.New("no migration files found")
	}

	return migrations, nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
create table if not exists schema_migrations (
  name text primary key,
  applied_at timestamptz not null default now()
)
`)
	return err
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, migrationName string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, "select exists(select 1 from schema_migrations where name = $1)", migrationName).Scan(&exists)
	return exists, err
}

func runSQLScript(ctx context.Context, pool *pgxpool.Pool, script string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	result := conn.Conn().PgConn().Exec(ctx, script)
	_, err = result.ReadAll()
	return err
}
