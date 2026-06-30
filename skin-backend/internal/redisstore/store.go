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
	BannedUntil *int64 `json:"banned_until,omitempty"`
}

func AuthUserFromModel(u model.User) AuthUser {
	return AuthUser{ID: u.ID, BannedUntil: u.BannedUntil}
}

func (u AuthUser) Banned(now time.Time) bool {
	return u.BannedUntil != nil && *u.BannedUntil > now.UnixMilli()
}

type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type OAuthAccessToken struct {
	TokenHash     string  `json:"token_hash"`
	ClientID      string  `json:"client_id"`
	UserID        string  `json:"user_id,omitempty"`
	GrantID       string  `json:"grant_id,omitempty"`
	PermissionIDs []int64 `json:"permission_ids"`
	ExpiresAt     int64   `json:"expires_at"`
	CreatedAt     int64   `json:"created_at"`
}

type Store interface {
	Close() error
	GetSetting(context.Context, string) (string, error)
	SetSetting(context.Context, string, string, time.Duration) error
	InvalidateSettings(context.Context) error
	GetPublicSettings(context.Context) (map[string]any, error)
	SetPublicSettings(context.Context, map[string]any, time.Duration) error
	InvalidatePublicSettings(context.Context) error
	GetPublicHomepageMedia(context.Context) ([]model.HomepageMedia, error)
	SetPublicHomepageMedia(context.Context, []model.HomepageMedia, time.Duration) error
	InvalidatePublicHomepageMedia(context.Context) error
	SetVerificationCode(context.Context, string, string, string, time.Duration) error
	SetVerificationCodeIfAbsent(context.Context, string, string, string, time.Duration) (bool, error)
	GetVerificationCode(context.Context, string, string) (string, error)
	ConsumeVerificationCode(context.Context, string, string, string) (bool, error)
	DeleteVerificationCode(context.Context, string, string) error
	SetYggToken(context.Context, model.Token, time.Duration) error
	GetYggToken(context.Context, string) (model.Token, error)
	ReplaceYggToken(context.Context, string, model.Token, time.Duration) (bool, error)
	DeleteYggToken(context.Context, string) error
	DeleteYggTokensByUser(context.Context, string) error
	TrimYggTokensByUser(context.Context, string, int) error
	SetYggSession(context.Context, model.Session, time.Duration) error
	GetYggSession(context.Context, string) (model.Session, error)
	MarkFallbackRequest(context.Context, string, string, time.Duration) (bool, error)
	DeleteFallbackRequest(context.Context, string, string) error
	SetState(context.Context, string, map[string]any, time.Duration) error
	PopState(context.Context, string) (map[string]any, error)
	HitRateLimit(context.Context, string, int, time.Duration) (RateLimitResult, error)
	GetAuthUser(context.Context, string) (AuthUser, error)
	SetAuthUser(context.Context, AuthUser, time.Duration) error
	InvalidateAuthUser(context.Context, string) error
	AppendProbeSamples(context.Context, []ProbeSample, time.Duration) error
	GetProbeHistory(context.Context, time.Time) ([]ProbeSample, error)
	InvalidateProbeHistory(context.Context) error
	DeleteByPrefix(context.Context, string) error
	GetPermissionCache(ctx context.Context, subjectID string) (string, bool, error)
	SetPermissionCache(ctx context.Context, subjectID string, encoded string, ttl time.Duration) error
	DeletePermissionCache(ctx context.Context, subjectID string) error
	SetOAuthAccessToken(context.Context, OAuthAccessToken, time.Duration) error
	GetOAuthAccessToken(context.Context, string) (OAuthAccessToken, error)
	DeleteOAuthAccessToken(context.Context, string) error
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
