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
	ID           string `json:"id"`
	IsAdmin      bool   `json:"is_admin"`
	IsSuperAdmin bool   `json:"is_super_admin"`
	BannedUntil  *int64 `json:"banned_until,omitempty"`
}

func AuthUserFromModel(u model.User) AuthUser {
	return AuthUser{ID: u.ID, IsAdmin: u.IsAdmin, IsSuperAdmin: u.IsSuperAdmin, BannedUntil: u.BannedUntil}
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
}

type yggToken struct {
	AccessToken string  `json:"access_token"`
	ClientToken string  `json:"client_token"`
	UserID      string  `json:"user_id"`
	ProfileID   *string `json:"profile_id,omitempty"`
	CreatedAt   int64   `json:"created_at"`
}

type yggSession struct {
	ServerID    string  `json:"server_id"`
	AccessToken string  `json:"access_token"`
	IP          *string `json:"ip,omitempty"`
	CreatedAt   int64   `json:"created_at"`
}

func yggTokenFromModel(t model.Token) yggToken {
	return yggToken{AccessToken: t.AccessToken, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: t.ProfileID, CreatedAt: t.CreatedAt}
}

func (t yggToken) model() model.Token {
	return model.Token{AccessToken: t.AccessToken, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: t.ProfileID, CreatedAt: t.CreatedAt}
}

func yggSessionFromModel(s model.Session) yggSession {
	return yggSession{ServerID: s.ServerID, AccessToken: s.AccessToken, IP: s.IP, CreatedAt: s.CreatedAt}
}

func (s yggSession) model() model.Session {
	return model.Session{ServerID: s.ServerID, AccessToken: s.AccessToken, IP: s.IP, CreatedAt: s.CreatedAt}
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

func (s *RedisStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *RedisStore) SetVerificationCode(ctx context.Context, email, typ, code string, ttl time.Duration) error {
	return s.client.Set(ctx, s.verificationKey(email, typ), code, ttl).Err()
}

func (s *RedisStore) SetVerificationCodeIfAbsent(ctx context.Context, email, typ, code string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.verificationKey(email, typ), code, ttl).Result()
}

func (s *RedisStore) GetVerificationCode(ctx context.Context, email, typ string) (string, error) {
	code, err := s.client.Get(ctx, s.verificationKey(email, typ)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return code, err
}

var consumeVerificationCodeScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value or string.lower(value) ~= string.lower(ARGV[1]) then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`)

func (s *RedisStore) ConsumeVerificationCode(ctx context.Context, email, typ, code string) (bool, error) {
	result, err := consumeVerificationCodeScript.Run(
		ctx,
		s.client,
		[]string{s.verificationKey(email, typ)},
		code,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *RedisStore) DeleteVerificationCode(ctx context.Context, email, typ string) error {
	return s.client.Del(ctx, s.verificationKey(email, typ)).Err()
}

func (s *RedisStore) yggTokenKey(access string) string {
	return s.key("ygg", "token", access)
}

func (s *RedisStore) yggUserTokensKey(userID string) string {
	return s.key("ygg", "user", userID, "tokens")
}

func (s *RedisStore) yggSessionKey(serverID string) string {
	return s.key("ygg", "session", serverID)
}

func (s *RedisStore) fallbackRequestKey(endpoint, request string) string {
	return s.key("fallback", "request", endpoint, request)
}

func (s *RedisStore) stateKey(token string) string {
	return s.key("state", token)
}

func (s *RedisStore) authUserKey(userID string) string {
	return s.key("auth", "user", "v2", userID)
}

func (s *RedisStore) SetYggToken(ctx context.Context, token model.Token, ttl time.Duration) error {
	value := yggTokenFromModel(token)
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.yggTokenKey(token.AccessToken), b, ttl)
	pipe.ZAdd(ctx, s.yggUserTokensKey(token.UserID), redis.Z{Score: float64(token.CreatedAt), Member: token.AccessToken})
	pipe.Expire(ctx, s.yggUserTokensKey(token.UserID), ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetYggToken(ctx context.Context, access string) (model.Token, error) {
	var token yggToken
	if err := s.getJSON(ctx, s.yggTokenKey(access), &token); err != nil {
		return model.Token{}, err
	}
	return token.model(), nil
}

var replaceYggTokenScript = redis.NewScript(`
local old = redis.call("GET", KEYS[1])
if not old then
  return 0
end
local decoded = cjson.decode(old)
redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[2])
redis.call("DEL", KEYS[1])
redis.call("ZREM", ARGV[3] .. decoded.user_id .. ARGV[4], ARGV[5])
redis.call("ZADD", KEYS[3], ARGV[6], ARGV[7])
redis.call("PEXPIRE", KEYS[3], ARGV[2])
return 1
`)

func (s *RedisStore) ReplaceYggToken(ctx context.Context, oldAccess string, token model.Token, ttl time.Duration) (bool, error) {
	value := yggTokenFromModel(token)
	b, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	res, err := replaceYggTokenScript.Run(ctx, s.client, []string{
		s.yggTokenKey(oldAccess),
		s.yggTokenKey(token.AccessToken),
		s.yggUserTokensKey(token.UserID),
	}, string(b), ttl.Milliseconds(), s.key("ygg", "user")+":", ":tokens", oldAccess, token.CreatedAt, token.AccessToken).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (s *RedisStore) DeleteYggToken(ctx context.Context, access string) error {
	token, err := s.GetYggToken(ctx, access)
	if errors.Is(err, ErrCacheMiss) {
		return nil
	}
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.yggTokenKey(access))
	pipe.ZRem(ctx, s.yggUserTokensKey(token.UserID), access)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) DeleteYggTokensByUser(ctx context.Context, userID string) error {
	key := s.yggUserTokensKey(userID)
	tokens, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	for _, access := range tokens {
		pipe.Del(ctx, s.yggTokenKey(access))
	}
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) TrimYggTokensByUser(ctx context.Context, userID string, keep int) error {
	if keep <= 0 {
		return s.DeleteYggTokensByUser(ctx, userID)
	}
	key := s.yggUserTokensKey(userID)
	count, err := s.client.ZCard(ctx, key).Result()
	if err != nil {
		return err
	}
	excess := count - int64(keep)
	if excess <= 0 {
		return nil
	}
	tokens, err := s.client.ZRange(ctx, key, 0, excess-1).Result()
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	for _, access := range tokens {
		pipe.Del(ctx, s.yggTokenKey(access))
		pipe.ZRem(ctx, key, access)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) SetYggSession(ctx context.Context, session model.Session, ttl time.Duration) error {
	return s.setJSON(ctx, s.yggSessionKey(session.ServerID), yggSessionFromModel(session), ttl)
}

func (s *RedisStore) GetYggSession(ctx context.Context, serverID string) (model.Session, error) {
	var session yggSession
	if err := s.getJSON(ctx, s.yggSessionKey(serverID), &session); err != nil {
		return model.Session{}, err
	}
	return session.model(), nil
}

func (s *RedisStore) MarkFallbackRequest(ctx context.Context, endpoint, request string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.fallbackRequestKey(endpoint, request), "1", ttl).Result()
}

func (s *RedisStore) DeleteFallbackRequest(ctx context.Context, endpoint, request string) error {
	return s.client.Del(ctx, s.fallbackRequestKey(endpoint, request)).Err()
}

func (s *RedisStore) SetState(ctx context.Context, token string, value map[string]any, ttl time.Duration) error {
	return s.setJSON(ctx, s.stateKey(token), value, ttl)
}

var popStateScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then
	return nil
end
redis.call("DEL", KEYS[1])
return value
`)

func (s *RedisStore) PopState(ctx context.Context, token string) (map[string]any, error) {
	result, err := popStateScript.Run(ctx, s.client, []string{s.stateKey(token)}).Result()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	var raw []byte
	switch value := result.(type) {
	case string:
		raw = []byte(value)
	case []byte:
		raw = value
	default:
		return nil, fmt.Errorf("unexpected state payload type %T", result)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
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
	if err := s.getJSON(ctx, s.authUserKey(userID), &out); err != nil {
		return AuthUser{}, err
	}
	return out, nil
}

func (s *RedisStore) SetAuthUser(ctx context.Context, user AuthUser, ttl time.Duration) error {
	return s.setJSON(ctx, s.authUserKey(user.ID), user, ttl)
}

func (s *RedisStore) InvalidateAuthUser(ctx context.Context, userID string) error {
	return s.client.Del(ctx, s.authUserKey(userID)).Err()
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
