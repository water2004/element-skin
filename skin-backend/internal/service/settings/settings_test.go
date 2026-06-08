package settings_test

import (
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsStoresDatabaseDependency(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db}
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
