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

func (s *MemoryStore) DeleteOAuthAccessTokensByClient(_ context.Context, clientID string) error {
	return s.deleteOAuthAccessTokens(func(token OAuthAccessToken) bool { return token.ClientID == clientID })
}

func (s *MemoryStore) DeleteOAuthAccessTokensByGrant(_ context.Context, grantID string) error {
	return s.deleteOAuthAccessTokens(func(token OAuthAccessToken) bool { return token.GrantID == grantID })
}

func (s *MemoryStore) DeleteOAuthAccessTokensByUser(_ context.Context, userID string) error {
	return s.deleteOAuthAccessTokens(func(token OAuthAccessToken) bool { return token.UserID == userID })
}

func (s *MemoryStore) deleteOAuthAccessTokens(match func(OAuthAccessToken) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for key, item := range s.items {
		if len(key) < len("oauth:access:") || key[:len("oauth:access:")] != "oauth:access:" {
			continue
		}
		raw, ok := item.value.(map[string]any)
		if !ok {
			token, ok := item.value.(OAuthAccessToken)
			if ok && match(token) {
				delete(s.items, key)
			}
			continue
		}
		if match(oauthAccessTokenFromMap(raw)) {
			delete(s.items, key)
		}
	}
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
	if values, ok := raw["oidc_scopes"].([]any); ok {
		token.OIDCScopes = make([]string, 0, len(values))
		for _, value := range values {
			if scope, ok := value.(string); ok {
				token.OIDCScopes = append(token.OIDCScopes, scope)
			}
		}
	} else if values, ok := raw["oidc_scopes"].([]string); ok {
		token.OIDCScopes = append([]string(nil), values...)
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
