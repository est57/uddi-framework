package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/storage"
)

var (
	ErrAPIKeyInvalid  = errors.New("invalid API key")
	ErrAPIKeyExists   = errors.New("api key already exists")
	ErrAPIKeyNotFound = errors.New("api key not found")
)

type APIKeyRecord struct {
	ServiceID   string `json:"serviceId"`
	ServiceName string `json:"serviceName"`
	CreatedAt   string `json:"createdAt"`
	RevokedAt   string `json:"revokedAt,omitempty"`
}

type APIKeyStore interface {
	Validate(ctx context.Context, serviceID string, apiKey string) error
	Create(ctx context.Context, serviceID string, serviceName string, apiKey string) (APIKeyRecord, error)
	List(ctx context.Context) ([]APIKeyRecord, error)
	Revoke(ctx context.Context, serviceID string) error
}

type MemoryAPIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]memoryAPIKeyRecord
}

type memoryAPIKeyRecord struct {
	record APIKeyRecord
	hash   string
}

func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	store := &MemoryAPIKeyStore{
		keys: make(map[string]memoryAPIKeyRecord),
	}
	_, _ = store.Create(context.Background(), "dev-service", "UDDI Dev Service", "dev-api-key")
	_, _ = store.Create(context.Background(), "test-service", "UDDI Test Service", "test-key")
	return store
}

func (s *MemoryAPIKeyStore) Add(serviceID string, apiKey string) {
	_, _ = s.Create(context.Background(), serviceID, serviceID, apiKey)
}

func (s *MemoryAPIKeyStore) Validate(_ context.Context, serviceID string, apiKey string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.keys[serviceID]
	if !ok || entry.record.RevokedAt != "" || entry.hash != hashAPIKey(apiKey) {
		return ErrAPIKeyInvalid
	}
	return nil
}

func (s *MemoryAPIKeyStore) Create(_ context.Context, serviceID string, serviceName string, apiKey string) (APIKeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[serviceID]; ok {
		return APIKeyRecord{}, ErrAPIKeyExists
	}
	record := APIKeyRecord{
		ServiceID:   serviceID,
		ServiceName: serviceName,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	s.keys[serviceID] = memoryAPIKeyRecord{
		record: record,
		hash:   hashAPIKey(apiKey),
	}
	return record, nil
}

func (s *MemoryAPIKeyStore) List(_ context.Context) ([]APIKeyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]APIKeyRecord, 0, len(s.keys))
	for _, entry := range s.keys {
		records = append(records, entry.record)
	}
	return records, nil
}

func (s *MemoryAPIKeyStore) Revoke(_ context.Context, serviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.keys[serviceID]
	if !ok {
		return ErrAPIKeyNotFound
	}
	if entry.record.RevokedAt == "" {
		entry.record.RevokedAt = time.Now().UTC().Format(time.RFC3339)
		s.keys[serviceID] = entry
	}
	return nil
}

type PostgresAPIKeyStore struct {
	db *sql.DB
}

func NewPostgresAPIKeyStore(ctx context.Context, databaseURL string) (*PostgresAPIKeyStore, error) {
	db, err := storage.OpenPostgres(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	store := &PostgresAPIKeyStore{db: db}
	if err := store.seed(ctx); err != nil {
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

func (s *PostgresAPIKeyStore) Create(ctx context.Context, serviceID string, serviceName string, apiKey string) (APIKeyRecord, error) {
	record := APIKeyRecord{
		ServiceID:   serviceID,
		ServiceName: serviceName,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (service_id, service_name, api_key_hash, created_at)
		VALUES ($1, $2, $3, $4)
	`, record.ServiceID, record.ServiceName, hashAPIKey(apiKey), record.CreatedAt)
	if err != nil {
		if isAPIKeyUniqueViolation(err) {
			return APIKeyRecord{}, ErrAPIKeyExists
		}
		return APIKeyRecord{}, err
	}
	return record, nil
}

func (s *PostgresAPIKeyStore) List(ctx context.Context) ([]APIKeyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT service_id, service_name, created_at, COALESCE(revoked_at, '')
		FROM api_keys
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]APIKeyRecord, 0)
	for rows.Next() {
		var record APIKeyRecord
		if err := rows.Scan(&record.ServiceID, &record.ServiceName, &record.CreatedAt, &record.RevokedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *PostgresAPIKeyStore) Revoke(ctx context.Context, serviceID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE service_id = $1
	`, serviceID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func (s *PostgresAPIKeyStore) seed(ctx context.Context) error {
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

func GenerateAPIKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "uddi_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func NormalizeAPIKeyInput(value string) string {
	return strings.TrimSpace(value)
}

func isAPIKeyUniqueViolation(err error) bool {
	var sqlState interface {
		SQLState() string
	}
	return errors.As(err, &sqlState) && sqlState.SQLState() == "23505"
}
