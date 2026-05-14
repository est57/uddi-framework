package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrAPIKeyInvalid = errors.New("invalid API key")

type APIKeyStore interface {
	Validate(ctx context.Context, serviceID string, apiKey string) error
}

type MemoryAPIKeyStore struct {
	keys map[string]string
}

func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	store := &MemoryAPIKeyStore{
		keys: make(map[string]string),
	}
	store.Add("dev-service", "dev-api-key")
	store.Add("test-service", "test-key")
	return store
}

func (s *MemoryAPIKeyStore) Add(serviceID string, apiKey string) {
	s.keys[serviceID] = hashAPIKey(apiKey)
}

func (s *MemoryAPIKeyStore) Validate(_ context.Context, serviceID string, apiKey string) error {
	expectedHash, ok := s.keys[serviceID]
	if !ok || expectedHash != hashAPIKey(apiKey) {
		return ErrAPIKeyInvalid
	}
	return nil
}

type PostgresAPIKeyStore struct {
	db *sql.DB
}

func NewPostgresAPIKeyStore(ctx context.Context, databaseURL string) (*PostgresAPIKeyStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	store := &PostgresAPIKeyStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresAPIKeyStore) Close() error {
	return s.db.Close()
}

func (s *PostgresAPIKeyStore) Validate(ctx context.Context, serviceID string, apiKey string) error {
	var storedHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT api_key_hash
		FROM api_keys
		WHERE service_id = $1
		  AND revoked_at IS NULL
	`, serviceID).Scan(&storedHash)
	if err != nil || storedHash != hashAPIKey(apiKey) {
		return ErrAPIKeyInvalid
	}
	return nil
}

func (s *PostgresAPIKeyStore) migrate(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS api_keys (
			service_id TEXT PRIMARY KEY,
			service_name TEXT NOT NULL,
			api_key_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			revoked_at TEXT
		)
	`); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	seeds := []struct {
		serviceID   string
		serviceName string
		apiKey      string
	}{
		{"dev-service", "UDDI Dev Service", "dev-api-key"},
		{"test-service", "UDDI Test Service", "test-key"},
	}

	for _, seed := range seeds {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO api_keys (service_id, service_name, api_key_hash, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (service_id) DO NOTHING
		`, seed.serviceID, seed.serviceName, hashAPIKey(seed.apiKey), now); err != nil {
			return err
		}
	}
	return nil
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}
