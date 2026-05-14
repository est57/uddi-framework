package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestPostgresMigrations(t *testing.T) {
	databaseURL := os.Getenv("UDDI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("UDDI_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migrate should be idempotent: %v", err)
	}

	for _, table := range []string{"schema_migrations", "dids", "api_keys", "credentials"} {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public'
				  AND table_name = $1
			)
		`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
