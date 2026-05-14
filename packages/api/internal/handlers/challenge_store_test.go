package handlers

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryChallengeStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryChallengeStore()
	challenge := authChallenge{
		ChallengeID: "challenge-1",
		Nonce:       "nonce-1",
		ServiceID:   "service-1",
		ServiceName: "Service One",
		IssuedAt:    time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339),
	}

	if err := store.Save(ctx, challenge); err != nil {
		t.Fatalf("save: %v", err)
	}

	resolved, err := store.Get(ctx, challenge.ChallengeID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resolved.Nonce != challenge.Nonce {
		t.Fatalf("expected nonce %s, got %s", challenge.Nonce, resolved.Nonce)
	}

	if err := store.Delete(ctx, challenge.ChallengeID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.Get(ctx, challenge.ChallengeID)
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Fatalf("expected ErrChallengeNotFound, got %v", err)
	}
}

func TestChallengeExpiresAtTime(t *testing.T) {
	expiresAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Second)
	challenge := authChallenge{
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}

	if !challenge.ExpiresAtTime().Equal(expiresAt) {
		t.Fatalf("expected %s, got %s", expiresAt, challenge.ExpiresAtTime())
	}
}
