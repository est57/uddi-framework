package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/storage"
)

var (
	ErrCredentialExists   = errors.New("credential already exists")
	ErrCredentialNotFound = errors.New("credential not found")
)

type CredentialRecord struct {
	ID               string         `json:"id"`
	IssuerDID        string         `json:"issuer"`
	SubjectDID       string         `json:"subject"`
	Types            []string       `json:"types"`
	Credential       map[string]any `json:"credential"`
	IssuanceDate     string         `json:"issuanceDate"`
	ExpirationDate   string         `json:"expirationDate,omitempty"`
	CreatedAt        string         `json:"createdAt"`
	RevokedAt        string         `json:"revokedAt,omitempty"`
	RevocationReason string         `json:"revocationReason,omitempty"`
}

type CredentialStore interface {
	Create(ctx context.Context, record CredentialRecord) error
	ListBySubject(ctx context.Context, did string) ([]CredentialRecord, error)
	Get(ctx context.Context, id string) (*CredentialRecord, error)
	Revoke(ctx context.Context, id string, reason string) error
}

type MemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]CredentialRecord
}

func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{
		credentials: make(map[string]CredentialRecord),
	}
}

func (s *MemoryCredentialStore) Create(_ context.Context, record CredentialRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.credentials[record.ID]; ok {
		return ErrCredentialExists
	}
	s.credentials[record.ID] = record
	return nil
}

func (s *MemoryCredentialStore) ListBySubject(_ context.Context, did string) ([]CredentialRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]CredentialRecord, 0)
	for _, record := range s.credentials {
		if record.SubjectDID == did {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s *MemoryCredentialStore) Get(_ context.Context, id string) (*CredentialRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.credentials[id]
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return &record, nil
}

func (s *MemoryCredentialStore) Revoke(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.credentials[id]
	if !ok {
		return ErrCredentialNotFound
	}
	if record.RevokedAt == "" {
		record.RevokedAt = time.Now().UTC().Format(time.RFC3339)
		record.RevocationReason = reason
		s.credentials[id] = record
	}
	return nil
}

type PostgresCredentialStore struct {
	db *sql.DB
}

func NewPostgresCredentialStore(ctx context.Context, databaseURL string) (*PostgresCredentialStore, error) {
	db, err := storage.OpenPostgres(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &PostgresCredentialStore{db: db}, nil
}

func (s *PostgresCredentialStore) Close() error {
	return s.db.Close()
}

func (s *PostgresCredentialStore) Create(ctx context.Context, record CredentialRecord) error {
	typesJSON, err := json.Marshal(record.Types)
	if err != nil {
		return err
	}
	credentialJSON, err := json.Marshal(record.Credential)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO credentials (
			id, issuer_did, subject_did, types, credential, issuance_date,
			expiration_date, created_at, revoked_at, revocation_reason
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''), NULLIF($10, ''))
	`, record.ID, record.IssuerDID, record.SubjectDID, string(typesJSON), string(credentialJSON), record.IssuanceDate,
		record.ExpirationDate, record.CreatedAt, record.RevokedAt, record.RevocationReason)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrCredentialExists
		}
		return err
	}
	return nil
}

func (s *PostgresCredentialStore) ListBySubject(ctx context.Context, did string) ([]CredentialRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, issuer_did, subject_did, types::text, credential::text, issuance_date,
		       COALESCE(expiration_date, ''), created_at, COALESCE(revoked_at, ''),
		       COALESCE(revocation_reason, '')
		FROM credentials
		WHERE subject_did = $1
		ORDER BY created_at DESC
	`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]CredentialRecord, 0)
	for rows.Next() {
		record, err := scanCredentialRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *PostgresCredentialStore) Get(ctx context.Context, id string) (*CredentialRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, issuer_did, subject_did, types::text, credential::text, issuance_date,
		       COALESCE(expiration_date, ''), created_at, COALESCE(revoked_at, ''),
		       COALESCE(revocation_reason, '')
		FROM credentials
		WHERE id = $1
	`, id)

	record, err := scanCredentialRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCredentialNotFound
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *PostgresCredentialStore) Revoke(ctx context.Context, id string, reason string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE credentials
		SET revoked_at = COALESCE(revoked_at, $2),
		    revocation_reason = COALESCE(revocation_reason, NULLIF($3, ''))
		WHERE id = $1
	`, id, time.Now().UTC().Format(time.RFC3339), reason)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

type credentialScanner interface {
	Scan(dest ...any) error
}

func scanCredentialRecord(scanner credentialScanner) (CredentialRecord, error) {
	var record CredentialRecord
	var typesJSON string
	var credentialJSON string

	if err := scanner.Scan(
		&record.ID,
		&record.IssuerDID,
		&record.SubjectDID,
		&typesJSON,
		&credentialJSON,
		&record.IssuanceDate,
		&record.ExpirationDate,
		&record.CreatedAt,
		&record.RevokedAt,
		&record.RevocationReason,
	); err != nil {
		return CredentialRecord{}, err
	}

	if err := json.Unmarshal([]byte(typesJSON), &record.Types); err != nil {
		return CredentialRecord{}, err
	}
	if err := json.Unmarshal([]byte(credentialJSON), &record.Credential); err != nil {
		return CredentialRecord{}, err
	}
	return record, nil
}

func isUniqueViolation(err error) bool {
	var sqlState interface {
		SQLState() string
	}
	return errors.As(err, &sqlState) && sqlState.SQLState() == "23505"
}
