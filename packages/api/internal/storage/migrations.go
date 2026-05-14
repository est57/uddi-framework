package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var Migrations = []Migration{
	{
		Version: 1,
		Name:    "create_dids",
		SQL: `
			CREATE TABLE IF NOT EXISTS dids (
				did TEXT PRIMARY KEY,
				context JSONB NOT NULL,
				public_key_base64 TEXT NOT NULL,
				created TEXT NOT NULL,
				updated TEXT NOT NULL,
				deactivated BOOLEAN NOT NULL DEFAULT FALSE
			)
		`,
	},
	{
		Version: 2,
		Name:    "create_api_keys",
		SQL: `
			CREATE TABLE IF NOT EXISTS api_keys (
				service_id TEXT PRIMARY KEY,
				service_name TEXT NOT NULL,
				api_key_hash TEXT NOT NULL,
				created_at TEXT NOT NULL,
				revoked_at TEXT
			)
		`,
	},
	{
		Version: 3,
		Name:    "create_credentials",
		SQL: `
			CREATE TABLE IF NOT EXISTS credentials (
				id TEXT PRIMARY KEY,
				issuer_did TEXT NOT NULL,
				subject_did TEXT NOT NULL,
				types JSONB NOT NULL,
				credential JSONB NOT NULL,
				issuance_date TEXT NOT NULL,
				expiration_date TEXT,
				created_at TEXT NOT NULL,
				revoked_at TEXT,
				revocation_reason TEXT
			);
			CREATE INDEX IF NOT EXISTS credentials_subject_did_idx
				ON credentials(subject_did);
			CREATE INDEX IF NOT EXISTS credentials_issuer_did_idx
				ON credentials(issuer_did);
		`,
	},
}

func OpenPostgres(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return err
	}

	for _, migration := range Migrations {
		applied, err := migrationApplied(ctx, db, migration.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version int) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM schema_migrations WHERE version = $1
		)
	`, version).Scan(&exists)
	return exists, err
}

func applyMigration(ctx context.Context, db *sql.DB, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES ($1, $2, $3)
	`, migration.Version, migration.Name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	var sqlState interface {
		SQLState() string
	}
	return errors.As(err, &sqlState) && sqlState.SQLState() == "23505"
}
