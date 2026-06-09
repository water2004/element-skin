package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestRedisStoreHelpersPreserveStableKeysAndModels(t *testing.T) {
	defaultStore := New(nil, "")
	if defaultStore.key("settings", "site_name") != "elementskin:settings:site_name" {
		t.Fatalf("default prefix key mismatch: %q", defaultStore.key("settings", "site_name"))
	}
	if err := defaultStore.Close(); err != nil {
		t.Fatalf("nil client close should be safe: %v", err)
	}

	customStore := New(nil, "tenant")
	if customStore.prefix != "tenant:" || customStore.key("public", "settings") != "tenant:public:settings" {
		t.Fatalf("custom prefix should be normalized exactly: prefix=%q key=%q", customStore.prefix, customStore.key("public", "settings"))
	}
	alreadyNormalized := New(nil, "tenant:")
	if alreadyNormalized.prefix != "tenant:" {
		t.Fatalf("already-normalized prefix should not gain another colon: %q", alreadyNormalized.prefix)
	}

	bannedUntil := time.Now().Add(time.Minute).UnixMilli()
	user := AuthUserFromModel(model.User{ID: "u1", IsAdmin: true, IsSuperAdmin: true, BannedUntil: &bannedUntil})
	if user.ID != "u1" || !user.IsAdmin || !user.IsSuperAdmin || user.BannedUntil == nil || !user.Banned(time.Now()) {
		t.Fatalf("auth user model conversion mismatch: %#v", user)
	}
	expiredBan := time.Now().Add(-time.Minute).UnixMilli()
	user.BannedUntil = &expiredBan
	if user.Banned(time.Now()) {
		t.Fatalf("expired ban should not be active: %#v", user)
	}

	profileID := "p1"
	token := model.Token{AccessToken: "access", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 123}
	roundTripToken := yggTokenFromModel(token).model()
	if roundTripToken.AccessToken != token.AccessToken || roundTripToken.ClientToken != token.ClientToken ||
		roundTripToken.UserID != token.UserID || roundTripToken.ProfileID == nil || *roundTripToken.ProfileID != profileID ||
		roundTripToken.CreatedAt != token.CreatedAt {
		t.Fatalf("ygg token conversion mismatch: %#v", roundTripToken)
	}

	ip := "203.0.113.10"
	session := model.Session{ServerID: "server", AccessToken: "access", IP: &ip, CreatedAt: 456}
	roundTripSession := yggSessionFromModel(session).model()
	if roundTripSession.ServerID != session.ServerID || roundTripSession.AccessToken != session.AccessToken ||
		roundTripSession.IP == nil || *roundTripSession.IP != ip || roundTripSession.CreatedAt != session.CreatedAt {
		t.Fatalf("ygg session conversion mismatch: %#v", roundTripSession)
	}
}

func TestMemoryStoreYggDeleteAndTrimExactLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	profileID := "p1"
	tokens := []model.Token{
		{AccessToken: "old", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 1},
		{AccessToken: "new", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 2},
	}
	for _, token := range tokens {
		if err := store.SetYggToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DeleteYggToken(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "old"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted token should miss, got %v", err)
	}
	remaining, err := store.GetYggToken(ctx, "new")
	if err != nil || remaining.AccessToken != "new" || remaining.UserID != "u1" {
		t.Fatalf("other token should remain after single delete: %#v err=%v", remaining, err)
	}
	if err := store.DeleteYggToken(ctx, "missing"); err != nil {
		t.Fatalf("deleting a missing token should be idempotent: %v", err)
	}

	replaced, err := store.ReplaceYggToken(ctx, "missing", model.Token{AccessToken: "replacement", UserID: "u1", CreatedAt: 3}, time.Minute)
	if err != nil || replaced {
		t.Fatalf("replacing a missing token should return false without error: replaced=%v err=%v", replaced, err)
	}
	if err := store.TrimYggTokensByUser(ctx, "u1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "new"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("trim keep=0 should delete all user tokens, got %v", err)
	}
	if err := store.TrimYggTokensByUser(ctx, "missing-user", 2); err != nil {
		t.Fatalf("trimming a missing user should be a no-op: %v", err)
	}
}

func TestMemoryStorePrefixTTLAndErrorContracts(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	now := time.UnixMilli(1_000)
	store.now = func() time.Time { return now }

	if err := store.SetSetting(ctx, "site_name", "A", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPublicCarousel(ctx, []string{"one.png"}, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteByPrefix(ctx, "settings:"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("prefixed delete should remove matching settings key, got %v", err)
	}
	if carousel, err := store.GetPublicCarousel(ctx); err != nil || len(carousel) != 1 || carousel[0] != "one.png" {
		t.Fatalf("prefixed delete should not remove unrelated carousel cache: %#v err=%v", carousel, err)
	}

	if err := store.SetVerificationCode(ctx, "ttl@example.com", "register", "12345678", time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := store.GetVerificationCode(ctx, "ttl@example.com", "register"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("verification code should expire at ttl boundary, got %v", err)
	}

	boom := errors.New("cache unavailable")
	store.Err = boom
	if err := store.SetSetting(ctx, "site_name", "B", time.Minute); !errors.Is(err, boom) {
		t.Fatalf("set should propagate store error, got %v", err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, boom) {
		t.Fatalf("get should propagate store error, got %v", err)
	}
	if err := store.InvalidateAuthUser(ctx, "u1"); !errors.Is(err, boom) {
		t.Fatalf("invalidate auth should propagate store error, got %v", err)
	}
	if err := store.DeleteFallbackRequest(ctx, "endpoint", "request"); !errors.Is(err, boom) {
		t.Fatalf("delete fallback guard should propagate store error, got %v", err)
	}
	if _, err := store.HitRateLimit(ctx, "login", 1, time.Minute); !errors.Is(err, boom) {
		t.Fatalf("rate limit should propagate store error, got %v", err)
	}
}
