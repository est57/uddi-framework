package middleware

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresAPIKeyStoreValidatesSeededDevKey(t *testing.T) {
	databaseURL := os.Getenv("UDDI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("UDDI_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresAPIKeyStore(ctx, databaseURL, true)
	if err != nil {
		t.Fatalf("new postgres API key store: %v", err)
	}
	defer store.Close()

	if err := store.Validate(ctx, "dev-service", "dev-api-key"); err != nil {
		t.Fatalf("validate dev key: %v", err)
	}

	err = store.Validate(ctx, "dev-service", "wrong-key")
	if !errors.Is(err, ErrAPIKeyInvalid) {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}
}
