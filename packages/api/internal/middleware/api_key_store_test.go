package middleware

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryAPIKeyStoreValidatesSeededKeys(t *testing.T) {
	store := NewMemoryAPIKeyStore()

	if err := store.Validate(context.Background(), "dev-service", "dev-api-key"); err != nil {
		t.Fatalf("validate dev key: %v", err)
	}
	if err := store.Validate(context.Background(), "test-service", "test-key"); err != nil {
		t.Fatalf("validate test key: %v", err)
	}
}

func TestMemoryAPIKeyStoreRejectsInvalidKeys(t *testing.T) {
	store := NewMemoryAPIKeyStore()

	err := store.Validate(context.Background(), "dev-service", "wrong-key")
	if !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}

	err = store.Validate(context.Background(), "unknown-service", "dev-api-key")
	if !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestMemoryAPIKeyStoreCreateListAndRevoke(t *testing.T) {
	store := NewMemoryAPIKeyStore()
	ctx := context.Background()

	record, err := store.Create(ctx, "new-service", "New Service", "new-api-key")
	if err != nil {
		t.Fatalf("create API key: %v", err)
	}
	if record.ServiceID != "new-service" {
		t.Fatalf("expected service ID new-service, got %s", record.ServiceID)
	}

	if err := store.Validate(ctx, "new-service", "new-api-key"); err != nil {
		t.Fatalf("validate created key: %v", err)
	}

	records, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list API keys: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records including seeded keys, got %d", len(records))
	}

	if err := store.Revoke(ctx, "new-service"); err != nil {
		t.Fatalf("revoke API key: %v", err)
	}

	err = store.Validate(ctx, "new-service", "new-api-key")
	if !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("expected revoked key to be invalid, got %v", err)
	}
}
