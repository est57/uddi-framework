package blockchain

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresDIDStore struct {
	db *sql.DB
}

func NewPostgresDIDStore(ctx context.Context, databaseURL string) (*PostgresDIDStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	store := &PostgresDIDStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresDIDStore) Close() error {
	return s.db.Close()
}

func (s *PostgresDIDStore) Exists(ctx context.Context, did string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dids WHERE did = $1)`, did).Scan(&exists)
	return exists, err
}

func (s *PostgresDIDStore) Create(ctx context.Context, doc DIDDocument) error {
	contextJSON, err := json.Marshal(doc.Context)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO dids (did, context, public_key_base64, created, updated, deactivated)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, doc.ID, string(contextJSON), doc.PublicKeyBase64, doc.Created, doc.Updated, doc.Deactivated)
	return err
}

func (s *PostgresDIDStore) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	var doc DIDDocument
	var contextJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT did, context::text, public_key_base64, created, updated, deactivated
		FROM dids
		WHERE did = $1
	`, did).Scan(&doc.ID, &contextJSON, &doc.PublicKeyBase64, &doc.Created, &doc.Updated, &doc.Deactivated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDIDNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(contextJSON), &doc.Context); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *PostgresDIDStore) Update(ctx context.Context, doc DIDDocument) error {
	contextJSON, err := json.Marshal(doc.Context)
	if err != nil {
		return err
	}

	if doc.Updated == "" {
		doc.Updated = time.Now().UTC().Format(time.RFC3339)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE dids
		SET context = $2,
			public_key_base64 = $3,
			created = $4,
			updated = $5,
			deactivated = $6
		WHERE did = $1
	`, doc.ID, string(contextJSON), doc.PublicKeyBase64, doc.Created, doc.Updated, doc.Deactivated)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrDIDNotFound
	}
	return nil
}

func (s *PostgresDIDStore) migrate(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dids (
			did TEXT PRIMARY KEY,
			context JSONB NOT NULL,
			public_key_base64 TEXT NOT NULL,
			created TEXT NOT NULL,
			updated TEXT NOT NULL,
			deactivated BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	return err
}
