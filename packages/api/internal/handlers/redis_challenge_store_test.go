package handlers

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRedisChallengeStoreLifecycle(t *testing.T) {
	redisURL := os.Getenv("UDDI_TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("UDDI_TEST_REDIS_URL is not set")
	}

	store, err := NewRedisChallengeStore(redisURL)
	if err != nil {
		t.Fatalf("new redis challenge store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	challenge := authChallenge{
		ChallengeID: "challenge-" + time.Now().UTC().Format("20060102150405.000000000"),
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
