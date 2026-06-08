package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("redis cache miss")

type AuthUser struct {
	ID          string `json:"id"`
	IsAdmin     bool   `json:"is_admin"`
	BannedUntil *int64 `json:"banned_until,omitempty"`
}

func AuthUserFromModel(u model.User) AuthUser {
	return AuthUser{ID: u.ID, IsAdmin: u.IsAdmin, BannedUntil: u.BannedUntil}
}

func (u AuthUser) Banned(now time.Time) bool {
	return u.BannedUntil != nil && *u.BannedUntil > now.UnixMilli()
}

type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type Store interface {
	Close() error
	GetSetting(context.Context, string) (string, error)
	SetSetting(context.Context, string, string, time.Duration) error
	InvalidateSettings(context.Context) error
	GetPublicSettings(context.Context) (map[string]any, error)
	SetPublicSettings(context.Context, map[string]any, time.Duration) error
	InvalidatePublicSettings(context.Context) error
	GetPublicCarousel(context.Context) ([]string, error)
	SetPublicCarousel(context.Context, []string, time.Duration) error
	InvalidatePublicCarousel(context.Context) error
	SetVerificationCode(context.Context, string, string, string, time.Duration) error
	GetVerificationCode(context.Context, string, string) (string, error)
	DeleteVerificationCode(context.Context, string, string) error
	HitRateLimit(context.Context, string, int, time.Duration) (RateLimitResult, error)
	GetAuthUser(context.Context, string) (AuthUser, error)
	SetAuthUser(context.Context, AuthUser, time.Duration) error
	InvalidateAuthUser(context.Context, string) error
	DeleteByPrefix(context.Context, string) error
}

type RedisStore struct {
	client *redis.Client
	prefix string
}

func Open(ctx context.Context, cfg config.Config) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis %s: %w", cfg.RedisAddr, err)
	}
	return New(client, cfg.RedisKeyPrefix), nil
}

func New(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{client: client, prefix: normalizePrefix(prefix)}
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) key(parts ...string) string {
	return s.prefix + strings.Join(parts, ":")
}

func normalizePrefix(prefix string) string {
	if prefix == "" {
		return "elementskin:"
	}
	if !strings.HasSuffix(prefix, ":") {
		return prefix + ":"
	}
	return prefix
}

func (s *RedisStore) getJSON(ctx context.Context, key string, dst any) error {
	raw, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func (s *RedisStore) setJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, b, ttl).Err()
}

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
	if err := s.getJSON(ctx, s.key("public", "settings"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RedisStore) SetPublicSettings(ctx context.Context, value map[string]any, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("public", "settings"), value, ttl)
}

func (s *RedisStore) InvalidatePublicSettings(ctx context.Context) error {
	return s.client.Del(ctx, s.key("public", "settings")).Err()
}

func (s *RedisStore) GetPublicCarousel(ctx context.Context) ([]string, error) {
	var out []string
	if err := s.getJSON(ctx, s.key("public", "carousel"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RedisStore) SetPublicCarousel(ctx context.Context, value []string, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("public", "carousel"), value, ttl)
}

func (s *RedisStore) InvalidatePublicCarousel(ctx context.Context) error {
	return s.client.Del(ctx, s.key("public", "carousel")).Err()
}

func (s *RedisStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *RedisStore) SetVerificationCode(ctx context.Context, email, typ, code string, ttl time.Duration) error {
	return s.client.Set(ctx, s.verificationKey(email, typ), code, ttl).Err()
}

func (s *RedisStore) GetVerificationCode(ctx context.Context, email, typ string) (string, error) {
	code, err := s.client.Get(ctx, s.verificationKey(email, typ)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return code, err
}

func (s *RedisStore) DeleteVerificationCode(ctx context.Context, email, typ string) error {
	return s.client.Del(ctx, s.verificationKey(email, typ)).Err()
}

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

func (s *RedisStore) HitRateLimit(ctx context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	if limit <= 0 {
		return RateLimitResult{Allowed: true}, nil
	}
	values, err := rateLimitScript.Run(ctx, s.client, []string{s.key("ratelimit", key)}, window.Milliseconds()).Slice()
	if err != nil {
		return RateLimitResult{}, err
	}
	count, _ := values[0].(int64)
	ttlMS, _ := values[1].(int64)
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{
		Allowed:    int(count) <= limit,
		Remaining:  remaining,
		RetryAfter: time.Duration(ttlMS) * time.Millisecond,
	}, nil
}

func (s *RedisStore) GetAuthUser(ctx context.Context, userID string) (AuthUser, error) {
	var out AuthUser
	if err := s.getJSON(ctx, s.key("auth", "user", userID), &out); err != nil {
		return AuthUser{}, err
	}
	return out, nil
}

func (s *RedisStore) SetAuthUser(ctx context.Context, user AuthUser, ttl time.Duration) error {
	return s.setJSON(ctx, s.key("auth", "user", user.ID), user, ttl)
}

func (s *RedisStore) InvalidateAuthUser(ctx context.Context, userID string) error {
	return s.client.Del(ctx, s.key("auth", "user", userID)).Err()
}

func (s *RedisStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	iter := s.client.Scan(ctx, 0, s.prefix+prefix+"*", 200).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 200 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return s.client.Del(ctx, keys...).Err()
	}
	return nil
}
