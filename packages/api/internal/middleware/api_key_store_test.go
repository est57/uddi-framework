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
