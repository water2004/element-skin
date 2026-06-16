package redisstore

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"element-skin/backend/internal/model"
)

type memoryItem struct {
	value     any
	expiresAt time.Time
}

type MemoryStore struct {
	mu     sync.Mutex
	now    func() time.Time
	items  map[string]memoryItem
	closed bool
	Err    error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{now: time.Now, items: map[string]memoryItem{}}
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *MemoryStore) key(parts ...string) string {
	return strings.Join(parts, ":")
}

func (s *MemoryStore) get(key string) (any, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	item, ok := s.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	if !item.expiresAt.IsZero() && !item.expiresAt.After(s.now()) {
		delete(s.items, key)
		return nil, ErrCacheMiss
	}
	return cloneValue(item.value), nil
}

func (s *MemoryStore) set(key string, value any, ttl time.Duration) error {
	if s.Err != nil {
		return s.Err
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.items[key] = memoryItem{value: cloneValue(value), expiresAt: expiresAt}
	return nil
}

func (s *MemoryStore) setPreservingExpiration(key string, value any) error {
	if s.Err != nil {
		return s.Err
	}
	item, ok := s.items[key]
	if !ok {
		return s.set(key, value, 0)
	}
	item.value = cloneValue(value)
	s.items[key] = item
	return nil
}

func cloneValue(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *MemoryStore) GetSetting(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("settings", key))
	if err != nil {
		return "", err
	}
	out, _ := v.(string)
	return out, nil
}

func (s *MemoryStore) SetSetting(_ context.Context, key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("settings", key), value, ttl)
}

func (s *MemoryStore) InvalidateSettings(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "settings:")
}

func (s *MemoryStore) GetPublicSettings(context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "settings"))
	if err != nil {
		return nil, err
	}
	if out, ok := v.(map[string]any); ok {
		return out, nil
	}
	return map[string]any{}, nil
}

func (s *MemoryStore) SetPublicSettings(_ context.Context, value map[string]any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "settings"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicSettings(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "settings"))
	return nil
}

func (s *MemoryStore) GetPublicHomepageMedia(context.Context) ([]model.HomepageMedia, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "homepage-media"))
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(v)
	var out []model.HomepageMedia
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = []model.HomepageMedia{}
	}
	return out, nil
}

func (s *MemoryStore) SetPublicHomepageMedia(_ context.Context, value []model.HomepageMedia, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "homepage-media"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicHomepageMedia(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "homepage-media"))
	return nil
}

func (s *MemoryStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *MemoryStore) SetVerificationCode(_ context.Context, email, typ, code string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.verificationKey(email, typ), code, ttl)
}

func (s *MemoryStore) SetVerificationCodeIfAbsent(_ context.Context, email, typ, code string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.verificationKey(email, typ)
	if _, err := s.get(key); err == nil {
		return false, nil
	} else if err != ErrCacheMiss {
		return false, err
	}
	return true, s.set(key, code, ttl)
}

func (s *MemoryStore) GetVerificationCode(_ context.Context, email, typ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.verificationKey(email, typ))
	if err != nil {
		return "", err
	}
	code, _ := v.(string)
	return code, nil
}

func (s *MemoryStore) ConsumeVerificationCode(_ context.Context, email, typ, code string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.verificationKey(email, typ)
	v, err := s.get(key)
	if err == ErrCacheMiss {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	stored, _ := v.(string)
	if !strings.EqualFold(stored, code) {
		return false, nil
	}
	delete(s.items, key)
	return true, nil
}

func (s *MemoryStore) DeleteVerificationCode(_ context.Context, email, typ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.verificationKey(email, typ))
	return nil
}

func (s *MemoryStore) yggTokenKey(access string) string {
	return s.key("ygg", "token", access)
}

func (s *MemoryStore) yggUserTokensKey(userID string) string {
	return s.key("ygg", "user", userID, "tokens")
}

func (s *MemoryStore) yggSessionKey(serverID string) string {
	return s.key("ygg", "session", serverID)
}

func (s *MemoryStore) authUserKey(userID string) string {
	return s.key("auth", "user", "v2", userID)
}

func (s *MemoryStore) SetYggToken(_ context.Context, token model.Token, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.set(s.yggTokenKey(token.AccessToken), token, ttl); err != nil {
		return err
	}
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	index[token.AccessToken] = token.CreatedAt
	return s.set(s.yggUserTokensKey(token.UserID), index, ttl)
}

func (s *MemoryStore) GetYggToken(_ context.Context, access string) (model.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getYggToken(access)
}

func (s *MemoryStore) getYggToken(access string) (model.Token, error) {
	v, err := s.get(s.yggTokenKey(access))
	if err != nil {
		return model.Token{}, err
	}
	b, _ := json.Marshal(v)
	var token model.Token
	_ = json.Unmarshal(b, &token)
	return token, nil
}

func (s *MemoryStore) ReplaceYggToken(_ context.Context, oldAccess string, token model.Token, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.getYggToken(oldAccess)
	if err == ErrCacheMiss {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.deleteYggToken(old); err != nil {
		return false, err
	}
	if err := s.set(s.yggTokenKey(token.AccessToken), token, ttl); err != nil {
		return false, err
	}
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return false, err
	}
	index[token.AccessToken] = token.CreatedAt
	if err := s.set(s.yggUserTokensKey(token.UserID), index, ttl); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MemoryStore) DeleteYggToken(_ context.Context, access string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.getYggToken(access)
	if err == ErrCacheMiss {
		return nil
	}
	if err != nil {
		return err
	}
	return s.deleteYggToken(token)
}

func (s *MemoryStore) deleteYggToken(token model.Token) error {
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.yggTokenKey(token.AccessToken))
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	delete(index, token.AccessToken)
	if len(index) == 0 {
		delete(s.items, s.yggUserTokensKey(token.UserID))
		return nil
	}
	return s.setPreservingExpiration(s.yggUserTokensKey(token.UserID), index)
}

func (s *MemoryStore) DeleteYggTokensByUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	index, err := s.yggTokenIndex(userID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	for access := range index {
		delete(s.items, s.yggTokenKey(access))
	}
	delete(s.items, s.yggUserTokensKey(userID))
	return nil
}

func (s *MemoryStore) TrimYggTokensByUser(_ context.Context, userID string, keep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if keep <= 0 {
		if s.Err != nil {
			return s.Err
		}
		index, err := s.yggTokenIndex(userID)
		if err != nil && err != ErrCacheMiss {
			return err
		}
		for access := range index {
			delete(s.items, s.yggTokenKey(access))
		}
		delete(s.items, s.yggUserTokensKey(userID))
		return nil
	}
	index, err := s.yggTokenIndex(userID)
	if err == ErrCacheMiss || len(index) <= keep {
		return nil
	}
	if err != nil {
		return err
	}
	type tokenRef struct {
		access    string
		createdAt int64
	}
	refs := make([]tokenRef, 0, len(index))
	for access, createdAt := range index {
		refs = append(refs, tokenRef{access: access, createdAt: createdAt})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].createdAt < refs[j].createdAt })
	for _, ref := range refs[:len(refs)-keep] {
		delete(s.items, s.yggTokenKey(ref.access))
		delete(index, ref.access)
	}
	return s.setPreservingExpiration(s.yggUserTokensKey(userID), index)
}

func (s *MemoryStore) yggTokenIndex(userID string) (map[string]int64, error) {
	v, err := s.get(s.yggUserTokensKey(userID))
	if err != nil {
		return map[string]int64{}, err
	}
	b, _ := json.Marshal(v)
	var index map[string]int64
	_ = json.Unmarshal(b, &index)
	if index == nil {
		index = map[string]int64{}
	}
	return index, nil
}

func (s *MemoryStore) SetYggSession(_ context.Context, session model.Session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.yggSessionKey(session.ServerID), session, ttl)
}

func (s *MemoryStore) GetYggSession(_ context.Context, serverID string) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.yggSessionKey(serverID))
	if err != nil {
		return model.Session{}, err
	}
	b, _ := json.Marshal(v)
	var session model.Session
	_ = json.Unmarshal(b, &session)
	return session, nil
}

func (s *MemoryStore) MarkFallbackRequest(_ context.Context, endpoint, request string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key("fallback", "request", endpoint, request)
	if _, err := s.get(key); err == nil {
		return false, nil
	} else if err != ErrCacheMiss {
		return false, err
	}
	return true, s.set(key, true, ttl)
}

func (s *MemoryStore) DeleteFallbackRequest(_ context.Context, endpoint, request string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("fallback", "request", endpoint, request))
	return nil
}

func (s *MemoryStore) HitRateLimit(_ context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return RateLimitResult{}, s.Err
	}
	if limit <= 0 {
		return RateLimitResult{Allowed: true}, nil
	}
	k := s.key("ratelimit", key)
	now := s.now()
	item, ok := s.items[k]
	count := 0
	expiresAt := now.Add(window)
	if ok && (item.expiresAt.IsZero() || item.expiresAt.After(now)) {
		if n, ok := item.value.(int); ok {
			count = n
		}
		expiresAt = item.expiresAt
	}
	count++
	s.items[k] = memoryItem{value: count, expiresAt: expiresAt}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{Allowed: count <= limit, Remaining: remaining, RetryAfter: expiresAt.Sub(now)}, nil
}

func (s *MemoryStore) GetAuthUser(_ context.Context, userID string) (AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.authUserKey(userID))
	if err != nil {
		return AuthUser{}, err
	}
	b, _ := json.Marshal(v)
	var out AuthUser
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *MemoryStore) SetAuthUser(_ context.Context, user AuthUser, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.authUserKey(user.ID), user, ttl)
}

func (s *MemoryStore) InvalidateAuthUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.authUserKey(userID))
	return nil
}

func (s *MemoryStore) probeHistoryKey() string {
	return s.key("probe", "history")
}

func (s *MemoryStore) AppendProbeSamples(_ context.Context, samples []ProbeSample, retention time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if len(samples) == 0 {
		return nil
	}
	key := s.probeHistoryKey()
	current := s.probeHistory(key)
	current = append(current, samples...)
	cutoff := s.now().Add(-retention).UnixMilli()
	if retention > 0 {
		filtered := current[:0]
		for _, sample := range current {
			if sample.CheckedAt >= cutoff {
				filtered = append(filtered, sample)
			}
		}
		current = filtered
	}
	sort.Slice(current, func(i, j int) bool { return current[i].CheckedAt < current[j].CheckedAt })
	return s.set(key, current, 0)
}

func (s *MemoryStore) GetProbeHistory(_ context.Context, since time.Time) ([]ProbeSample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	all := s.probeHistory(s.probeHistoryKey())
	cutoff := since.UnixMilli()
	out := make([]ProbeSample, 0, len(all))
	for _, sample := range all {
		if sample.CheckedAt >= cutoff {
			out = append(out, sample)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CheckedAt < out[j].CheckedAt })
	return out, nil
}

func (s *MemoryStore) InvalidateProbeHistory(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.probeHistoryKey())
	return nil
}

func (s *MemoryStore) probeHistory(key string) []ProbeSample {
	v, err := s.get(key)
	if err != nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var out []ProbeSample
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *MemoryStore) DeleteByPrefix(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for key := range s.items {
		if strings.HasPrefix(key, prefix) {
			delete(s.items, key)
		}
	}
	return nil
}
