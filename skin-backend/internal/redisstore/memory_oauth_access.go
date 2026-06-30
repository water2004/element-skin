package redisstore

import (
	"context"
	"time"
)

func (s *MemoryStore) SetOAuthAccessToken(_ context.Context, token OAuthAccessToken, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set("oauth:access:"+token.TokenHash, token, ttl)
}

func (s *MemoryStore) GetOAuthAccessToken(_ context.Context, tokenHash string) (OAuthAccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get("oauth:access:" + tokenHash)
	if err != nil {
		return OAuthAccessToken{}, err
	}
	raw, ok := v.(map[string]any)
	if !ok {
		token, _ := v.(OAuthAccessToken)
		return token, nil
	}
	return oauthAccessTokenFromMap(raw), nil
}

func (s *MemoryStore) DeleteOAuthAccessToken(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, "oauth:access:"+tokenHash)
	return nil
}

func oauthAccessTokenFromMap(raw map[string]any) OAuthAccessToken {
	token := OAuthAccessToken{
		TokenHash: stringValue(raw["token_hash"]),
		ClientID:  stringValue(raw["client_id"]),
		UserID:    stringValue(raw["user_id"]),
		GrantID:   stringValue(raw["grant_id"]),
		ExpiresAt: int64Value(raw["expires_at"]),
		CreatedAt: int64Value(raw["created_at"]),
	}
	if values, ok := raw["permission_ids"].([]any); ok {
		token.PermissionIDs = make([]int64, 0, len(values))
		for _, value := range values {
			token.PermissionIDs = append(token.PermissionIDs, int64Value(value))
		}
	}
	return token
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
