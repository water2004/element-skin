package redisstore

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPublicSettingsCacheUsesOriginalKeyAndInvalidatesIt(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := t.Context()
	key := store.key("public", "settings")
	want := map[string]any{"site_name": "Current"}
	if err := store.SetPublicSettings(ctx, want, time.Minute); err != nil {
		t.Fatal(err)
	}
	if raw, err := server.Get(key); err != nil || raw != `{"site_name":"Current"}` {
		t.Fatalf("public cache raw value=%q err=%v; want exact JSON at %q", raw, err, key)
	}
	if ttl := server.TTL(key); ttl != time.Minute {
		t.Fatalf("public cache TTL=%v; want %v", ttl, time.Minute)
	}
	if cached, err := store.GetPublicSettings(ctx); err != nil || !reflect.DeepEqual(cached, want) {
		t.Fatalf("public cache=%#v err=%v; want %#v", cached, err, want)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if server.Exists(key) {
		t.Fatalf("invalidation retained public cache key %q", key)
	}
	if cached, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) || cached != nil {
		t.Fatalf("invalidated public cache=%#v err=%v; want nil and ErrCacheMiss", cached, err)
	}
}
