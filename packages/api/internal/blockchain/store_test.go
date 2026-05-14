package blockchain

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
)

func TestMemoryDIDStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryDIDStore()
	doc := DIDDocument{
		Context:         []string{"https://www.w3.org/ns/did/v1"},
		ID:              "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		PublicKeyBase64: base64.StdEncoding.EncodeToString([]byte("public-key")),
		Created:         "2026-05-13T00:00:00Z",
		Updated:         "2026-05-13T00:00:00Z",
	}

	exists, err := store.Exists(ctx, doc.ID)
	if err != nil {
		t.Fatalf("exists before create: %v", err)
	}
	if exists {
		t.Fatal("expected DID not to exist before create")
	}

	if err := store.Create(ctx, doc); err != nil {
		t.Fatalf("create: %v", err)
	}

	resolved, err := store.Resolve(ctx, doc.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.ID != doc.ID {
		t.Fatalf("expected DID %s, got %s", doc.ID, resolved.ID)
	}

	resolved.Deactivated = true
	if err := store.Update(ctx, *resolved); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated, err := store.Resolve(ctx, doc.ID)
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
	if stats.TotalDIDs != 1 || stats.ActiveDIDs != 0 || stats.DeactivatedDIDs != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.Backend != "memory" {
		t.Fatalf("expected memory backend, got %s", stats.Backend)
	}
}

func TestMemoryDIDStoreReturnsNotFound(t *testing.T) {
	store := NewMemoryDIDStore()

	_, err := store.Resolve(context.Background(), "did:uddi:zmissing")
	if !errors.Is(err, ErrDIDNotFound) {
		t.Fatalf("expected ErrDIDNotFound, got %v", err)
	}
}

func TestClientUsesStore(t *testing.T) {
	ctx := context.Background()
	client := NewClientWithStore("memory://test", NewMemoryDIDStore())
	did := "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij"

	txHash, err := client.RegisterDID(ctx, RegisterDIDParams{
		DID:       did,
		PublicKey: []byte("public-key"),
	})
	if err != nil {
		t.Fatalf("register DID: %v", err)
	}
	if txHash == "" {
		t.Fatal("expected tx hash")
	}

	exists, err := client.DIDExists(ctx, did)
	if err != nil {
		t.Fatalf("DID exists: %v", err)
	}
	if !exists {
		t.Fatal("expected DID to exist")
	}

	if err := client.RevokeDID(ctx, did); err != nil {
		t.Fatalf("revoke DID: %v", err)
	}

	doc, err := client.ResolveDID(ctx, did)
	if err != nil {
		t.Fatalf("resolve DID: %v", err)
	}
	if !doc.Deactivated {
		t.Fatal("expected DID to be deactivated")
	}

	stats, err := client.RegistryStats(ctx)
	if err != nil {
		t.Fatalf("registry stats: %v", err)
	}
	if stats.TotalDIDs != 1 || stats.DeactivatedDIDs != 1 {
		t.Fatalf("unexpected registry stats: %+v", stats)
	}
}

func TestClientUpdatesDID(t *testing.T) {
	ctx := context.Background()
	client := NewClientWithStore("memory://test", NewMemoryDIDStore())
	did := "did:uddi:zupdate123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdef"

	_, err := client.RegisterDID(ctx, RegisterDIDParams{
		DID:       did,
		PublicKey: []byte("old-public-key"),
	})
	if err != nil {
		t.Fatalf("register DID: %v", err)
	}

	txHash, err := client.UpdateDID(ctx, UpdateDIDParams{
		DID:       did,
		PublicKey: []byte("new-public-key"),
		Context:   []string{"https://www.w3.org/ns/did/v1", "https://uddi.network/v1"},
	})
	if err != nil {
		t.Fatalf("update DID: %v", err)
	}
	if txHash == "" {
		t.Fatal("expected update tx hash")
	}

	doc, err := client.ResolveDID(ctx, did)
	if err != nil {
		t.Fatalf("resolve DID: %v", err)
	}
	if doc.PublicKeyBase64 != base64.StdEncoding.EncodeToString([]byte("new-public-key")) {
		t.Fatalf("expected updated public key, got %s", doc.PublicKeyBase64)
	}
	if len(doc.Context) != 2 {
		t.Fatalf("expected updated DID context, got %+v", doc.Context)
	}
}
