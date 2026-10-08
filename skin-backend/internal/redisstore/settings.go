package redisstore

import (
	"context"
	"time"

	"element-skin/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

const publicSettingsCacheKey = "settings:v2"

func (s *RedisStore) GetSetting(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, s.key("settings", key)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return value, err
}

func (s *RedisStore) SetSetting(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.client.Set(ctx, s.key("settings", key), value, ttl).Err()
}

func (s *RedisStore) InvalidateSettings(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "settings:")
}

func (s *RedisStore) GetPublicSettings(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.getJSON(ctx, s.key("public", publicSettingsCacheKey), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RedisStore) SetPublicSettings(ctx context.Context, value map[string]any, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("public", publicSettingsCacheKey), value, ttl)
}

func (s *RedisStore) InvalidatePublicSettings(ctx context.Context) error {
	return s.client.Del(ctx, s.key("public", publicSettingsCacheKey)).Err()
}

func (s *RedisStore) GetPublicHomepageMedia(ctx context.Context) ([]model.HomepageMedia, error) {
	var out []model.HomepageMedia
	if err := s.getJSON(ctx, s.key("public", "homepage-media"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RedisStore) SetPublicHomepageMedia(ctx context.Context, value []model.HomepageMedia, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("public", "homepage-media"), value, ttl)
}

func (s *RedisStore) InvalidatePublicHomepageMedia(ctx context.Context) error {
	return s.client.Del(ctx, s.key("public", "homepage-media")).Err()
}
