package blockchain

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresDIDStoreLifecycle(t *testing.T) {
	databaseURL := os.Getenv("UDDI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("UDDI_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresDIDStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new postgres DID store: %v", err)
	}
	defer store.Close()

	did := "did:uddi:ztest" + time.Now().UTC().Format("20060102150405.000000000")
	doc := DIDDocument{
		Context:         []string{"https://www.w3.org/ns/did/v1", "https://uddi.network/v1"},
		ID:              did,
		PublicKeyBase64: base64.StdEncoding.EncodeToString([]byte("postgres-public-key")),
		Created:         time.Now().UTC().Format(time.RFC3339),
		Updated:         time.Now().UTC().Format(time.RFC3339),
		Deactivated:     false,
	}

	exists, err := store.Exists(ctx, did)
	if err != nil {
		t.Fatalf("exists before create: %v", err)
	}
	if exists {
		t.Fatal("expected DID not to exist before create")
	}

	if err := store.Create(ctx, doc); err != nil {
		t.Fatalf("create: %v", err)
	}

	resolved, err := store.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.ID != did {
		t.Fatalf("expected DID %s, got %s", did, resolved.ID)
	}

	resolved.Deactivated = true
	if err := store.Update(ctx, *resolved); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated, err := store.Resolve(ctx, did)
	if err != nil {
		t.Fatalf("resolve updated: %v", err)
	}
	if !updated.Deactivated {
		t.Fatal("expected DID to be deactivated")
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalDIDs < 1 {
		t.Fatalf("expected at least one DID, got %+v", stats)
	}
	if stats.Backend != "postgres" {
		t.Fatalf("expected postgres backend, got %s", stats.Backend)
	}
}

func TestPostgresDIDStoreNotFound(t *testing.T) {
	databaseURL := os.Getenv("UDDI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("UDDI_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresDIDStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new postgres DID store: %v", err)
	}
	defer store.Close()

	_, err = store.Resolve(ctx, "did:uddi:zmissing")
	if !errors.Is(err, ErrDIDNotFound) {
		t.Fatalf("expected ErrDIDNotFound, got %v", err)
	}
}
