package settings_test

import (
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsStoresDatabaseDependency(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if settings.DB != db {
		t.Fatalf("Settings should retain DB dependency")
	}
}

func TestSettingsReadsThroughRedisCacheAndInvalidates(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: redis}

	if err := db.Settings.Set(t.Context(), "site_name", "Cached Name"); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Get(t.Context(), "site_name", "")
	if err != nil || first != "Cached Name" {
		t.Fatalf("first settings read mismatch: %q err=%v", first, err)
	}
	if err := db.Settings.Set(t.Context(), "site_name", "DB Changed"); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.Get(t.Context(), "site_name", "")
	if err != nil || cached != "Cached Name" {
		t.Fatalf("setting should be served from redis cache: %q err=%v", cached, err)
	}
	if err := svc.InvalidateCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	refreshed, err := svc.Get(t.Context(), "site_name", "")
	if err != nil || refreshed != "DB Changed" {
		t.Fatalf("setting should refresh after invalidation: %q err=%v", refreshed, err)
	}
}

func TestSettingsReturnsRedisErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := redisstore.NewMemoryStore()
	redis.Err = errors.New("redis unavailable")
	svc := settings.Settings{DB: db, Redis: redis}

	if _, err := svc.Get(t.Context(), "site_name", "fallback"); err == nil || err.Error() != "redis unavailable" {
		t.Fatalf("settings should return redis error, got %v", err)
	}
}

func TestSettingsIntUsesCacheTTLAndFallbacksExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := redisstore.NewMemoryStore()
	svc := settings.Settings{DB: db, Redis: redis, TTL: 500 * time.Millisecond}

	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "7"); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Int(t.Context(), "rate_limit_auth_attempts", 5)
	if err != nil || first != 7 {
		t.Fatalf("first Int read mismatch: got=%d err=%v", first, err)
	}
	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "9"); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.Int(t.Context(), "rate_limit_auth_attempts", 5)
	if err != nil || cached != 7 {
		t.Fatalf("Int should use redis value before custom TTL expires: got=%d err=%v", cached, err)
	}
	time.Sleep(700 * time.Millisecond)
	refreshed, err := svc.Int(t.Context(), "rate_limit_auth_attempts", 5)
	if err != nil || refreshed != 9 {
		t.Fatalf("Int should refresh after custom TTL expires: got=%d err=%v", refreshed, err)
	}

	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "bad-number"); err != nil {
		t.Fatal(err)
	}
	if err := svc.InvalidateCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	fallback, err := svc.Int(t.Context(), "rate_limit_auth_attempts", 5)
	if err != nil || fallback != 5 {
		t.Fatalf("Int should return fallback for invalid stored number: got=%d err=%v", fallback, err)
	}
}
