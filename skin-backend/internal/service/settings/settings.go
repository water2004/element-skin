package settings

import (
	"context"
	"errors"
	"strconv"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
)

const CacheTTL = 10 * time.Minute

type Settings struct {
	DB    *database.DB
	Redis redisstore.Store
	TTL   time.Duration
}

func (s Settings) cacheTTL() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return CacheTTL
}

func (s Settings) Get(ctx context.Context, key, fallback string) (string, error) {
	if s.Redis == nil {
		return s.DB.Settings.Get(ctx, key, fallback)
	}
	value, err := s.Redis.GetSetting(ctx, key)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, redisstore.ErrCacheMiss) {
		return "", err
	}
	value, err = s.DB.Settings.Get(ctx, key, fallback)
	if err != nil {
		return "", err
	}
	return value, s.Redis.SetSetting(ctx, key, value, s.cacheTTL())
}

func (s Settings) Int(ctx context.Context, key string, fallback int) (int, error) {
	value, err := s.Get(ctx, key, strconv.Itoa(fallback))
	if err != nil {
		return fallback, err
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback, nil
	}
	return n, nil
}

func (s Settings) InvalidateCache(ctx context.Context) error {
	if s.Redis == nil {
		return nil
	}
	return s.Redis.InvalidateSettings(ctx)
}
