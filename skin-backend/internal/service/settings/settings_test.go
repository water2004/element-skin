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

func TestSettingsBoolMatchesManagementValuesAndPreservesErrors(t *testing.T) {
	cache := redisstore.NewMemoryStore()
	svc := settings.Settings{Redis: cache}
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"true", true},
		{"1", false},
		{"TRUE", false},
		{" true ", false},
		{"false", false},
		{"0", false},
		{"", false},
		{"invalid", false},
	} {
		if err := cache.SetSetting(t.Context(), "enable_strong_password_check", tc.raw, time.Minute); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Bool(t.Context(), "enable_strong_password_check", true)
		if err != nil || got != tc.want {
			t.Fatalf("Bool(%q)=%t err=%v; want %t", tc.raw, got, err, tc.want)
		}
	}
	cache.Err = errors.New("bool cache unavailable")
	got, err := svc.Bool(t.Context(), "enable_strong_password_check", true)
	if !got || !errors.Is(err, cache.Err) {
		t.Fatalf("Bool cache failure=%t err=%v; want true and exact cache error", got, err)
	}
}

func TestSettingsBoolUsesMissingValueFallbackAndInvalidation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := redisstore.NewMemoryStore()
	svc := settings.Settings{DB: db, Redis: cache}
	const key = "test_bool_fallback"
	got, err := svc.Bool(t.Context(), key, true)
	if err != nil || !got {
		t.Fatalf("missing bool=%t err=%v; want true fallback", got, err)
	}
	if raw, err := cache.GetSetting(t.Context(), key); err != nil || raw != "true" {
		t.Fatalf("cached fallback=%q err=%v; want true", raw, err)
	}
	if err := svc.InvalidateCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Bool(t.Context(), key, false)
	if err != nil || got {
		t.Fatalf("invalidated missing bool=%t err=%v; want false fallback", got, err)
	}
	if raw, err := cache.GetSetting(t.Context(), key); err != nil || raw != "false" {
		t.Fatalf("cached refreshed fallback=%q err=%v; want false", raw, err)
	}
}
