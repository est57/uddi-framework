package blockchain

import (
	"context"
	"sync"
	"time"
)

type DIDStore interface {
	Exists(ctx context.Context, did string) (bool, error)
	Create(ctx context.Context, doc DIDDocument) error
	Resolve(ctx context.Context, did string) (*DIDDocument, error)
	Update(ctx context.Context, doc DIDDocument) error
	Stats(ctx context.Context) (RegistryStats, error)
}

type RegistryStats struct {
	TotalDIDs       int64  `json:"totalDIDs"`
	ActiveDIDs      int64  `json:"activeDIDs"`
	DeactivatedDIDs int64  `json:"deactivatedDIDs"`
	Backend         string `json:"backend"`
}

type MemoryDIDStore struct {
	mu   sync.RWMutex
	dids map[string]DIDDocument
}

func NewMemoryDIDStore() *MemoryDIDStore {
	return &MemoryDIDStore{
		dids: make(map[string]DIDDocument),
	}
}

func (s *MemoryDIDStore) Exists(_ context.Context, did string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.dids[did]
	return ok, nil
}

func (s *MemoryDIDStore) Create(_ context.Context, doc DIDDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dids[doc.ID] = doc
	return nil
}

func (s *MemoryDIDStore) Resolve(_ context.Context, did string) (*DIDDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.dids[did]
	if !ok {
		return nil, ErrDIDNotFound
	}
	return &doc, nil
}

func (s *MemoryDIDStore) Update(_ context.Context, doc DIDDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dids[doc.ID]; !ok {
		return ErrDIDNotFound
	}
	doc.Updated = time.Now().UTC().Format(time.RFC3339)
	s.dids[doc.ID] = doc
	return nil
}

func (s *MemoryDIDStore) Stats(_ context.Context) (RegistryStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := RegistryStats{Backend: "memory"}
	for _, doc := range s.dids {
		stats.TotalDIDs++
		if doc.Deactivated {
			stats.DeactivatedDIDs++
			continue
		}
		stats.ActiveDIDs++
	}
	return stats, nil
}
