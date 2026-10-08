package redisstore

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPublicSettingsCacheVersionDoesNotReadRemovedPrivateFields(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := t.Context()
	previousKey := store.key("public", "settings")
	previous := `{"site_name":"Previous","enable_strong_password_check":true}`
	if err := server.Set(previousKey, previous); err != nil {
		t.Fatal(err)
	}
	server.SetTTL(previousKey, time.Minute)
	if cached, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) || cached != nil {
		t.Fatalf("new public cache read previous schema: got=%#v err=%v", cached, err)
	}
	want := map[string]any{"site_name": "Current"}
	if err := store.SetPublicSettings(ctx, want, time.Minute); err != nil {
		t.Fatal(err)
	}
	if cached, err := store.GetPublicSettings(ctx); err != nil || !reflect.DeepEqual(cached, want) {
		t.Fatalf("new public cache=%#v; want=%#v err=%v", cached, want, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if cached, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) || cached != nil {
		t.Fatalf("invalidated current public cache=%#v err=%v", cached, err)
	}
	if previousValue, err := server.Get(previousKey); err != nil || previousValue != previous {
		t.Fatalf("current public cache operations changed previous key: got=%q err=%v", previousValue, err)
	}
}
