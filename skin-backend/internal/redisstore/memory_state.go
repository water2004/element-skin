package redisstore

import (
	"context"
	"encoding/json"
	"time"
)

func (s *MemoryStore) stateKey(token string) string {
	return s.key("state", token)
}

func (s *MemoryStore) SetState(_ context.Context, token string, value map[string]any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.stateKey(token), value, ttl)
}

func (s *MemoryStore) GetState(_ context.Context, token string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.get(s.stateKey(token))
	if err != nil {
		return nil, err
	}
	return memoryStateMap(value), nil
}

func (s *MemoryStore) PopState(_ context.Context, token string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.stateKey(token)
	value, err := s.get(key)
	if err != nil {
		return nil, err
	}
	delete(s.items, key)
	return memoryStateMap(value), nil
}

func (s *MemoryStore) DeleteState(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.stateKey(token))
	return nil
}

func memoryStateMap(value any) map[string]any {
	out, ok := value.(map[string]any)
	if ok {
		return out
	}
	b, _ := json.Marshal(value)
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}
