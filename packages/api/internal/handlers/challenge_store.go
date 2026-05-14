package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrChallengeNotFound = errors.New("challenge not found")

type ChallengeStore interface {
	Save(ctx context.Context, challenge authChallenge) error
	Get(ctx context.Context, challengeID string) (authChallenge, error)
	Delete(ctx context.Context, challengeID string) error
}

type MemoryChallengeStore struct {
	mu         sync.RWMutex
	challenges map[string]authChallenge
}

func NewMemoryChallengeStore() *MemoryChallengeStore {
	return &MemoryChallengeStore{
		challenges: make(map[string]authChallenge),
	}
}

func (s *MemoryChallengeStore) Save(_ context.Context, challenge authChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges[challenge.ChallengeID] = challenge
	return nil
}

func (s *MemoryChallengeStore) Get(_ context.Context, challengeID string) (authChallenge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return authChallenge{}, ErrChallengeNotFound
	}
	return challenge, nil
}

func (s *MemoryChallengeStore) Delete(_ context.Context, challengeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.challenges, challengeID)
	return nil
}

type RedisChallengeStore struct {
	client *redis.Client
}

func NewRedisChallengeStore(redisURL string) (*RedisChallengeStore, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &RedisChallengeStore{
		client: redis.NewClient(options),
	}, nil
}

func (s *RedisChallengeStore) Close() error {
	return s.client.Close()
}

func (s *RedisChallengeStore) Save(ctx context.Context, challenge authChallenge) error {
	payload, err := json.Marshal(challenge)
	if err != nil {
		return err
	}

	ttl := time.Until(challenge.ExpiresAtTime())
	if ttl <= 0 {
		ttl = time.Second
	}
	return s.client.Set(ctx, challengeKey(challenge.ChallengeID), payload, ttl).Err()
}

func (s *RedisChallengeStore) Get(ctx context.Context, challengeID string) (authChallenge, error) {
	payload, err := s.client.Get(ctx, challengeKey(challengeID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return authChallenge{}, ErrChallengeNotFound
	}
	if err != nil {
		return authChallenge{}, err
	}

	var challenge authChallenge
	if err := json.Unmarshal(payload, &challenge); err != nil {
		return authChallenge{}, err
	}
	return challenge, nil
}

func (s *RedisChallengeStore) Delete(ctx context.Context, challengeID string) error {
	return s.client.Del(ctx, challengeKey(challengeID)).Err()
}

func challengeKey(challengeID string) string {
	return "uddi:auth_challenge:" + challengeID
}

func (c authChallenge) ExpiresAtTime() time.Time {
	expiresAt, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return time.Now().UTC()
	}
	return expiresAt
}
