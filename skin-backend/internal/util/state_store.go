package util

import (
	"sync"
	"time"
)

type stateItem struct {
	value     any
	expiresAt time.Time
}

type InMemoryStateStore struct {
	mu   sync.Mutex
	data map[string]stateItem
}

func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{data: map[string]stateItem{}}
}

func (s *InMemoryStateStore) Put(key string, value any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.data[key] = stateItem{value: value, expiresAt: time.Now().Add(ttl)}
}

func (s *InMemoryStateStore) Pop(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.data[key]
	if !ok {
		return nil
	}
	delete(s.data, key)
	if time.Now().After(item.expiresAt) {
		return nil
	}
	return item.value
}

func (s *InMemoryStateStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

func (s *InMemoryStateStore) sweepLocked() {
	now := time.Now()
	for k, item := range s.data {
		if now.After(item.expiresAt) {
			delete(s.data, k)
		}
	}
}
