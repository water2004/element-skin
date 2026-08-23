package redisstore

import (
	"context"
	"time"
)

func (s *MemoryStore) SetExternalAccessToken(_ context.Context, token ExternalAccessToken, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set("identity:access:"+token.IdentityID, token, ttl)
}

func (s *MemoryStore) GetExternalAccessToken(_ context.Context, identityID string) (ExternalAccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.get("identity:access:" + identityID)
	if err != nil {
		return ExternalAccessToken{}, err
	}
	if token, ok := value.(ExternalAccessToken); ok {
		return token, nil
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return ExternalAccessToken{}, ErrCacheMiss
	}
	return ExternalAccessToken{
		IdentityID:  stringValue(raw["identity_id"]),
		AccessToken: stringValue(raw["access_token"]),
		TokenType:   stringValue(raw["token_type"]),
		ExpiresAt:   int64Value(raw["expires_at"]),
	}, nil
}

func (s *MemoryStore) DeleteExternalAccessToken(_ context.Context, identityID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, "identity:access:"+identityID)
	return nil
}
