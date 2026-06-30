package redisstore

import (
	"context"
	"time"
)

func (s *RedisStore) SetOAuthAccessToken(ctx context.Context, token OAuthAccessToken, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("oauth", "access", token.TokenHash), token, ttl)
}

func (s *RedisStore) GetOAuthAccessToken(ctx context.Context, tokenHash string) (OAuthAccessToken, error) {
	var token OAuthAccessToken
	if err := s.getJSON(ctx, s.key("oauth", "access", tokenHash), &token); err != nil {
		return OAuthAccessToken{}, err
	}
	return token, nil
}

func (s *RedisStore) DeleteOAuthAccessToken(ctx context.Context, tokenHash string) error {
	return s.client.Del(ctx, s.key("oauth", "access", tokenHash)).Err()
}
