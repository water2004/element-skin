package redisstore

import (
	"context"
	"time"
)

func (s *RedisStore) SetExternalAccessToken(ctx context.Context, token ExternalAccessToken, ttl time.Duration) error {
	return s.setJSON(ctx, s.externalAccessKey(token.IdentityID), token, ttl)
}

func (s *RedisStore) GetExternalAccessToken(ctx context.Context, identityID string) (ExternalAccessToken, error) {
	var token ExternalAccessToken
	if err := s.getJSON(ctx, s.externalAccessKey(identityID), &token); err != nil {
		return ExternalAccessToken{}, err
	}
	return token, nil
}

func (s *RedisStore) DeleteExternalAccessToken(ctx context.Context, identityID string) error {
	return s.client.Del(ctx, s.externalAccessKey(identityID)).Err()
}

func (s *RedisStore) externalAccessKey(identityID string) string {
	return s.key("identity", "access", identityID)
}
