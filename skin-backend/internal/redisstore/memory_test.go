package redisstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreCachesAndInvalidatesPublicData(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty setting should miss, got %v", err)
	}
	if err := store.SetSetting(ctx, "site_name", "Cached Setting", time.Minute); err != nil {
		t.Fatal(err)
	}
	setting, err := store.GetSetting(ctx, "site_name")
	if err != nil || setting != "Cached Setting" {
		t.Fatalf("setting cache mismatch: %q err=%v", setting, err)
	}
	if err := store.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated setting should miss, got %v", err)
	}

	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty settings should miss, got %v", err)
	}
	if err := store.SetPublicSettings(ctx, map[string]any{"site_name": "Cached"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPublicSettings(ctx)
	if err != nil || got["site_name"] != "Cached" {
		t.Fatalf("settings cache mismatch: %#v err=%v", got, err)
	}
	got["site_name"] = "mutated"
	again, _ := store.GetPublicSettings(ctx)
	if again["site_name"] != "Cached" {
		t.Fatalf("cache should return cloned data, got %#v", again)
	}
	if err := store.SetPublicCarousel(ctx, []string{"a.png"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	carousel, err := store.GetPublicCarousel(ctx)
	if err != nil || len(carousel) != 1 || carousel[0] != "a.png" {
		t.Fatalf("carousel cache mismatch: %#v err=%v", carousel, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated settings should miss, got %v", err)
	}
}

func TestMemoryStoreVerificationRateLimitAndAuthCache(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	if err := store.SetVerificationCode(ctx, "User@Example.com", "register", "ABC12345", time.Minute); err != nil {
		t.Fatal(err)
	}
	code, err := store.GetVerificationCode(ctx, "user@example.com", "register")
	if err != nil || code != "ABC12345" {
		t.Fatalf("verification code mismatch: %q err=%v", code, err)
	}
	if err := store.DeleteVerificationCode(ctx, "user@example.com", "register"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("deleted code should miss, got %v", err)
	}

	for i := 0; i < 2; i++ {
		res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
		if err != nil || !res.Allowed {
			t.Fatalf("hit %d should be allowed: %#v err=%v", i+1, res, err)
		}
	}
	res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
	if err != nil || res.Allowed || res.Remaining != 0 {
		t.Fatalf("third hit should be rejected: %#v err=%v", res, err)
	}

	until := time.Now().Add(time.Hour).UnixMilli()
	auth := redisstore.AuthUser{ID: "u1", IsAdmin: true, BannedUntil: &until}
	if err := store.SetAuthUser(ctx, auth, time.Minute); err != nil {
		t.Fatal(err)
	}
	cached, err := store.GetAuthUser(ctx, "u1")
	if err != nil || !cached.IsAdmin || !cached.Banned(time.Now()) {
		t.Fatalf("auth cache mismatch: %#v err=%v", cached, err)
	}
	if err := store.InvalidateAuthUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAuthUser(ctx, "u1"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated auth cache should miss, got %v", err)
	}
}

func TestMemoryStoreYggTokenLifecycleAndTrim(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	profileID := "p1"

	for i := 1; i <= 4; i++ {
		if err := store.SetYggToken(ctx, model.Token{
			AccessToken: "access_" + string(rune('0'+i)),
			ClientToken: "client",
			UserID:      "u1",
			ProfileID:   &profileID,
			CreatedAt:   int64(i),
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.TrimYggTokensByUser(ctx, "u1", 2); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_1", "access_2"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be trimmed, got %v", access, err)
		}
	}
	for _, access := range []string{"access_3", "access_4"} {
		token, err := store.GetYggToken(ctx, access)
		if err != nil || token.UserID != "u1" || token.ProfileID == nil || *token.ProfileID != profileID {
			t.Fatalf("%s should remain: %#v err=%v", access, token, err)
		}
	}

	replaced, err := store.ReplaceYggToken(ctx, "access_3", model.Token{
		AccessToken: "access_new",
		ClientToken: "client",
		UserID:      "u1",
		ProfileID:   &profileID,
		CreatedAt:   5,
	}, time.Minute)
	if err != nil || !replaced {
		t.Fatalf("replace should succeed: replaced=%v err=%v", replaced, err)
	}
	if _, err := store.GetYggToken(ctx, "access_3"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old token should miss after replace, got %v", err)
	}
	if token, err := store.GetYggToken(ctx, "access_new"); err != nil || token.UserID != "u1" {
		t.Fatalf("new token mismatch: %#v err=%v", token, err)
	}

	if err := store.DeleteYggTokensByUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_4", "access_new"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be deleted by user, got %v", access, err)
		}
	}
}

func TestMemoryStoreYggSessionTTL(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	if err := store.SetYggSession(ctx, model.Session{ServerID: "server", AccessToken: "access", CreatedAt: 1}, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.GetYggSession(ctx, "server"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("expired ygg session should miss, got %v", err)
	}
}
