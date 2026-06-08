package redisstore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type memoryItem struct {
	value     any
	expiresAt time.Time
}

type MemoryStore struct {
	mu     sync.Mutex
	now    func() time.Time
	items  map[string]memoryItem
	closed bool
	Err    error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{now: time.Now, items: map[string]memoryItem{}}
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *MemoryStore) key(parts ...string) string {
	return strings.Join(parts, ":")
}

func (s *MemoryStore) get(key string) (any, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	item, ok := s.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	if !item.expiresAt.IsZero() && !item.expiresAt.After(s.now()) {
		delete(s.items, key)
		return nil, ErrCacheMiss
	}
	return cloneValue(item.value), nil
}

func (s *MemoryStore) set(key string, value any, ttl time.Duration) error {
	if s.Err != nil {
		return s.Err
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.items[key] = memoryItem{value: cloneValue(value), expiresAt: expiresAt}
	return nil
}

func cloneValue(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *MemoryStore) GetSetting(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("settings", key))
	if err != nil {
		return "", err
	}
	out, _ := v.(string)
	return out, nil
}

func (s *MemoryStore) SetSetting(_ context.Context, key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("settings", key), value, ttl)
}

func (s *MemoryStore) InvalidateSettings(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "settings:")
}

func (s *MemoryStore) GetPublicSettings(context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "settings"))
	if err != nil {
		return nil, err
	}
	if out, ok := v.(map[string]any); ok {
		return out, nil
	}
	return map[string]any{}, nil
}

func (s *MemoryStore) SetPublicSettings(_ context.Context, value map[string]any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "settings"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicSettings(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "settings"))
	return nil
}

func (s *MemoryStore) GetPublicCarousel(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "carousel"))
	if err != nil {
		return nil, err
	}
	raw, ok := v.([]any)
	if ok {
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, nil
	}
	if out, ok := v.([]string); ok {
		return out, nil
	}
	return []string{}, nil
}

func (s *MemoryStore) SetPublicCarousel(_ context.Context, value []string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "carousel"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicCarousel(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "carousel"))
	return nil
}

func (s *MemoryStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *MemoryStore) SetVerificationCode(_ context.Context, email, typ, code string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.verificationKey(email, typ), code, ttl)
}

func (s *MemoryStore) GetVerificationCode(_ context.Context, email, typ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.verificationKey(email, typ))
	if err != nil {
		return "", err
	}
	code, _ := v.(string)
	return code, nil
}

func (s *MemoryStore) DeleteVerificationCode(_ context.Context, email, typ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.verificationKey(email, typ))
	return nil
}

func (s *MemoryStore) HitRateLimit(_ context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return RateLimitResult{}, s.Err
	}
	if limit <= 0 {
		return RateLimitResult{Allowed: true}, nil
	}
	k := s.key("ratelimit", key)
	now := s.now()
	item, ok := s.items[k]
	count := 0
	expiresAt := now.Add(window)
	if ok && (item.expiresAt.IsZero() || item.expiresAt.After(now)) {
		if n, ok := item.value.(int); ok {
			count = n
		}
		expiresAt = item.expiresAt
	}
	count++
	s.items[k] = memoryItem{value: count, expiresAt: expiresAt}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{Allowed: count <= limit, Remaining: remaining, RetryAfter: expiresAt.Sub(now)}, nil
}

func (s *MemoryStore) GetAuthUser(_ context.Context, userID string) (AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("auth", "user", userID))
	if err != nil {
		return AuthUser{}, err
	}
	b, _ := json.Marshal(v)
	var out AuthUser
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *MemoryStore) SetAuthUser(_ context.Context, user AuthUser, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("auth", "user", user.ID), user, ttl)
}

func (s *MemoryStore) InvalidateAuthUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("auth", "user", userID))
	return nil
}

func (s *MemoryStore) DeleteByPrefix(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for key := range s.items {
		if strings.HasPrefix(key, prefix) {
			delete(s.items, key)
		}
	}
	return nil
}
