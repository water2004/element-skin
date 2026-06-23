package redisstore

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/model"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisStoreCacheRoundTripsNormalizationAndTTL(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()

	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing setting error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetSetting(ctx, "site_name", "Redis Site", time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetSetting(ctx, "site_name"); err != nil || got != "Redis Site" {
		t.Fatalf("setting=%q err=%v, want Redis Site", got, err)
	}

	public := map[string]any{"site_name": "Redis Site", "allow_register": true}
	if err := store.SetPublicSettings(ctx, public, time.Minute); err != nil {
		t.Fatal(err)
	}
	gotPublic, err := store.GetPublicSettings(ctx)
	if err != nil || gotPublic["site_name"] != "Redis Site" || gotPublic["allow_register"] != true {
		t.Fatalf("public settings=%#v err=%v", gotPublic, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated public settings error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetPublicSettings(ctx, public, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "one", Type: "image", StoragePath: "one.png"}, {ID: "two", Type: "image", StoragePath: "two.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetPublicHomepageMedia(ctx); err != nil || len(got) != 2 || got[0].StoragePath != "one.png" || got[1].StoragePath != "two.png" {
		t.Fatalf("homepage media=%#v err=%v", got, err)
	}

	if err := store.SetVerificationCode(ctx, " User@Example.com ", " RESET ", "ABC12345", time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || got != "ABC12345" {
		t.Fatalf("normalized verification code=%q err=%v", got, err)
	}
	if consumed, err := store.ConsumeVerificationCode(ctx, "USER@EXAMPLE.COM", "RESET", "wrong"); err != nil || consumed {
		t.Fatalf("wrong verification consumption=%v err=%v, want false nil", consumed, err)
	}
	if consumed, err := store.ConsumeVerificationCode(ctx, "USER@EXAMPLE.COM", "RESET", "abc12345"); err != nil || !consumed {
		t.Fatalf("matching verification consumption=%v err=%v, want true nil", consumed, err)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("consumed verification code error=%v, want ErrCacheMiss", err)
	}
	if set, err := store.SetVerificationCodeIfAbsent(ctx, "user@example.com", "reset", "NEWCODE1", time.Minute); err != nil || !set {
		t.Fatalf("set missing verification code=%v err=%v, want true nil", set, err)
	}
	if set, err := store.SetVerificationCodeIfAbsent(ctx, "user@example.com", "reset", "OLDCODE1", time.Minute); err != nil || set {
		t.Fatalf("set existing verification code=%v err=%v, want false nil", set, err)
	}
	if got, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || got != "NEWCODE1" {
		t.Fatalf("set-if-absent overwrote code=%q err=%v", got, err)
	}

	until := time.Now().Add(time.Hour).UnixMilli()
	auth := AuthUser{ID: "user-1", IsAdmin: true, IsSuperAdmin: false, BannedUntil: &until}
	if err := store.SetAuthUser(ctx, auth, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetAuthUser(ctx, auth.ID); err != nil || got.ID != auth.ID || !got.IsAdmin ||
		got.IsSuperAdmin || got.BannedUntil == nil || *got.BannedUntil != until {
		t.Fatalf("auth cache=%#v err=%v", got, err)
	}
	if err := store.InvalidateAuthUser(ctx, auth.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAuthUser(ctx, auth.ID); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated auth cache error=%v, want ErrCacheMiss", err)
	}

	ip := "203.0.113.7"
	session := model.Session{ServerID: "server-1", AccessToken: "access-1", IP: &ip, CreatedAt: 123}
	if err := store.SetYggSession(ctx, session, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetYggSession(ctx, session.ServerID); err != nil || got.ServerID != session.ServerID ||
		got.AccessToken != session.AccessToken || got.IP == nil || *got.IP != ip || got.CreatedAt != session.CreatedAt {
		t.Fatalf("session=%#v err=%v", got, err)
	}

	server.FastForward(time.Minute)
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("public settings should expire at TTL boundary, got %v", err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("homepage media should expire at TTL boundary, got %v", err)
	}
	if _, err := store.GetYggSession(ctx, session.ServerID); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("session should expire at TTL boundary, got %v", err)
	}
}

func TestRedisStoreStateIsAtomicSingleUseAndExpiresExactly(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()

	if _, err := store.PopState(ctx, "missing-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing state error=%v, want ErrCacheMiss", err)
	}

	state := map[string]any{
		"kind":    "oauth_state",
		"user_id": "user-1",
		"profile": map[string]any{
			"id":   "profile-1",
			"name": "PlayerOne",
		},
	}
	if err := store.SetState(ctx, "oauth-token", state, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.PopState(ctx, "oauth-token")
	if err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "oauth_state" || got["user_id"] != "user-1" {
		t.Fatalf("state scalar fields mismatch: %#v", got)
	}
	profile, ok := got["profile"].(map[string]any)
	if !ok || profile["id"] != "profile-1" || profile["name"] != "PlayerOne" {
		t.Fatalf("state nested profile mismatch: %#v", got)
	}
	if _, err := store.PopState(ctx, "oauth-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should be consumed atomically, got %v", err)
	}

	if err := store.SetState(ctx, "expires-token", map[string]any{"kind": "profile"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	if _, err := store.PopState(ctx, "expires-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should expire at TTL boundary, got %v", err)
	}
}

func TestRedisStoreYggTokenAtomicLifecycleAndIndexes(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	profileID := "profile-1"
	if err := store.TrimYggTokensByUser(ctx, "missing-user", 2); err != nil {
		t.Fatalf("trimming a missing user should be a no-op: %v", err)
	}
	tokens := []model.Token{
		{AccessToken: "access-1", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 1},
		{AccessToken: "access-2", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 2},
		{AccessToken: "access-3", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 3},
		{AccessToken: "access-4", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 4},
	}
	for _, token := range tokens {
		if err := store.SetYggToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := store.GetYggToken(ctx, "access-2"); err != nil || got.UserID != "user-1" ||
		got.ProfileID == nil || *got.ProfileID != profileID || got.CreatedAt != 2 {
		t.Fatalf("stored token=%#v err=%v", got, err)
	}

	replacement := model.Token{AccessToken: "access-new", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 5}
	replaced, err := store.ReplaceYggToken(ctx, "access-2", replacement, time.Minute)
	if err != nil || !replaced {
		t.Fatalf("replace result=%v err=%v, want true nil", replaced, err)
	}
	if _, err := store.GetYggToken(ctx, "access-2"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("old token should be removed atomically, got %v", err)
	}
	if got, err := store.GetYggToken(ctx, replacement.AccessToken); err != nil || got.CreatedAt != replacement.CreatedAt {
		t.Fatalf("replacement token=%#v err=%v", got, err)
	}
	if replaced, err := store.ReplaceYggToken(ctx, "missing", replacement, time.Minute); err != nil || replaced {
		t.Fatalf("replace missing result=%v err=%v, want false nil", replaced, err)
	}

	if err := store.TrimYggTokensByUser(ctx, "user-1", 2); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access-1", "access-3"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, ErrCacheMiss) {
			t.Fatalf("%s should be trimmed, got %v", access, err)
		}
	}
	for _, access := range []string{"access-4", "access-new"} {
		if got, err := store.GetYggToken(ctx, access); err != nil || got.AccessToken != access {
			t.Fatalf("newest token %s should remain: token=%#v err=%v", access, got, err)
		}
	}

	if err := store.DeleteYggToken(ctx, "access-new"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteYggToken(ctx, "access-new"); err != nil {
		t.Fatalf("deleting a missing token should be idempotent: %v", err)
	}
	if err := store.DeleteYggTokensByUser(ctx, "user-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "access-4"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("user token deletion should remove remaining tokens, got %v", err)
	}

	finalToken := model.Token{AccessToken: "access-final", ClientToken: "client", UserID: "user-1", CreatedAt: 6}
	if err := store.SetYggToken(ctx, finalToken, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.TrimYggTokensByUser(ctx, "user-1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, finalToken.AccessToken); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("keep=0 should remove every token, got %v", err)
	}
}

func TestRedisStoreRateLimitFallbackGuardAndPrefixDeletion(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()

	if result, err := store.HitRateLimit(ctx, "disabled", 0, time.Minute); err != nil || !result.Allowed || result.Remaining != 0 {
		t.Fatalf("disabled limit result=%#v err=%v", result, err)
	}
	for hit, wantRemaining := range []int{1, 0, 0} {
		result, err := store.HitRateLimit(ctx, "login:ip:203.0.113.8", 2, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		wantAllowed := hit < 2
		if result.Allowed != wantAllowed || result.Remaining != wantRemaining ||
			result.RetryAfter <= 0 || result.RetryAfter > time.Minute {
			t.Fatalf("hit %d result=%#v, want allowed=%v remaining=%d and bounded retry", hit+1, result, wantAllowed, wantRemaining)
		}
	}

	first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute)
	if err != nil || !first {
		t.Fatalf("first fallback mark=%v err=%v, want true nil", first, err)
	}
	duplicate, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute)
	if err != nil || duplicate {
		t.Fatalf("duplicate fallback mark=%v err=%v, want false nil", duplicate, err)
	}
	if err := store.DeleteFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc"); err != nil {
		t.Fatal(err)
	}
	if first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute); err != nil || !first {
		t.Fatalf("deleted fallback guard should be reusable: first=%v err=%v", first, err)
	}

	for i := 0; i < 205; i++ {
		key := "bulk_" + strconv.Itoa(i)
		if err := store.SetSetting(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "keep", Type: "image", StoragePath: "keep.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		key := "bulk_" + strconv.Itoa(i)
		if _, err := store.GetSetting(ctx, key); !errors.Is(err, ErrCacheMiss) {
			t.Fatalf("setting %s should be deleted by prefix, got %v", key, err)
		}
	}
	if got, err := store.GetPublicHomepageMedia(ctx); err != nil || len(got) != 1 || got[0].StoragePath != "keep.png" {
		t.Fatalf("prefix deletion must preserve unrelated keys: homepage media=%#v err=%v", got, err)
	}
	if err := store.InvalidatePublicHomepageMedia(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated homepage media error=%v, want ErrCacheMiss", err)
	}
}

func TestRedisStoreConcurrentRateLimitAllowsExactThreshold(t *testing.T) {
	store, _ := newTestRedisStore(t)
	const attempts = 25
	const limit = 7
	type result struct {
		value RateLimitResult
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := store.HitRateLimit(context.Background(), "concurrent-redis", limit, time.Minute)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	rejected := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent Redis rate limit failed: %v", result.err)
		}
		if result.value.Allowed {
			allowed++
		} else {
			rejected++
		}
		if result.value.Remaining < 0 || result.value.Remaining > limit-1 ||
			result.value.RetryAfter <= 0 || result.value.RetryAfter > time.Minute {
			t.Fatalf("concurrent Redis rate limit returned invalid metadata: %#v", result.value)
		}
	}
	if allowed != limit || rejected != attempts-limit {
		t.Fatalf("concurrent Redis rate limit allowed=%d rejected=%d; want %d and %d", allowed, rejected, limit, attempts-limit)
	}
	final, err := store.HitRateLimit(context.Background(), "concurrent-redis", limit, time.Minute)
	if err != nil || final.Allowed || final.Remaining != 0 {
		t.Fatalf("final Redis rate-limit state=%#v err=%v; want rejected with zero remaining", final, err)
	}
}

func TestRedisStoreRejectsCorruptAndUnencodableJSON(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()
	publicKey := store.key("public", "settings")
	server.Set(publicKey, "{not-json")
	if got, err := store.GetPublicSettings(ctx); err == nil || got != nil {
		t.Fatalf("corrupt cached JSON should return a decode error: got=%#v err=%v", got, err)
	}

	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if err := store.setJSON(ctx, publicKey, cyclic, time.Minute); err == nil {
		t.Fatal("cyclic JSON value should be rejected before writing to Redis")
	}
	if got, err := server.Get(publicKey); err != nil || got != "{not-json" {
		t.Fatalf("failed JSON encoding must not overwrite existing cache: value=%q err=%v", got, err)
	}
}

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	cfg := config.Defaults()
	cfg.RedisAddr = server.Addr()
	cfg.RedisPassword = ""
	cfg.RedisDB = 0
	cfg.RedisKeyPrefix = "redisstore:test:"
	store, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close redis store: %v", err)
		}
	})
	return store, server
}
