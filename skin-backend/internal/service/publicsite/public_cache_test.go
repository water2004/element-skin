package publicsite_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	publicsvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestPublicSettingsAcceptsOnlyCompletePublicCacheFields(t *testing.T) {
	ctx := t.Context()
	cache := redisstore.NewMemoryStore()
	want := publicSettingsCacheFixture()
	if err := cache.SetPublicSettings(ctx, want, time.Minute); err != nil {
		t.Fatal(err)
	}
	// A valid cache hit must work without touching a database.
	got, err := (publicsvc.Service{Redis: cache}).PublicSettings(ctx, permission.GuestActor())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("cached public settings=%#v err=%v; want %#v", got, err, want)
	}
}

func TestPublicSettingsRejectsEveryMissingOrNonPublicCacheField(t *testing.T) {
	ctx := t.Context()
	boom := errors.New("public settings invalidation failed")
	cases := map[string]func(map[string]any){
		"private true":  func(values map[string]any) { values["enable_strong_password_check"] = true },
		"private false": func(values map[string]any) { values["enable_strong_password_check"] = false },
		"unknown":       func(values map[string]any) { values["internal_setting"] = "private" },
		"same size replacement": func(values map[string]any) {
			delete(values, "site_name")
			values["enable_strong_password_check"] = true
		},
	}
	for field := range publicSettingsCacheFixture() {
		cases["missing "+field] = func(values map[string]any) { delete(values, field) }
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cache := redisstore.NewMemoryStore()
			payload := publicSettingsCacheFixture()
			mutate(payload)
			if err := cache.SetPublicSettings(ctx, payload, time.Minute); err != nil {
				t.Fatal(err)
			}
			store := &publicCacheInvalidationFailure{Store: cache, err: boom}
			got, err := (publicsvc.Service{Redis: store}).PublicSettings(ctx, permission.GuestActor())
			if got != nil || !errors.Is(err, boom) || store.calls != 1 {
				t.Fatalf("settings=%#v err=%v invalidations=%d; want nil, exact failure, 1", got, err, store.calls)
			}
			cached, err := cache.GetPublicSettings(ctx)
			if err != nil || !reflect.DeepEqual(cached, payload) {
				t.Fatalf("failed invalidation changed cache: got=%#v err=%v; want %#v", cached, err, payload)
			}
		})
	}
}

func TestPublicSettingsRebuildsCacheContainingPrivateFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	cache := redisstore.NewMemoryStore()
	svc := publicsvc.Service{
		DB: db, Redis: cache, Settings: settingssvc.Settings{DB: db, Redis: cache},
		SiteURL: "https://site.example", APIURL: "https://api.example", CacheTTL: time.Minute,
	}
	want, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	// Normalize the fresh response to its JSON representation used by the cache.
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		payload := publicSettingsCacheFixture()
		payload["enable_strong_password_check"] = enabled
		if err := cache.SetPublicSettings(ctx, payload, time.Hour); err != nil {
			t.Fatal(err)
		}
		got, err := svc.PublicSettings(ctx, permission.GuestActor())
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var normalized map[string]any
		if err := json.Unmarshal(encoded, &normalized); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(normalized, want) {
			t.Fatalf("rebuilt settings=%#v; want %#v", normalized, want)
		}
		cached, err := cache.GetPublicSettings(ctx)
		if err != nil || !reflect.DeepEqual(cached, want) {
			t.Fatalf("rebuilt cache=%#v err=%v; want %#v", cached, err, want)
		}
		// A subsequent hit must return exactly the same public fields.
		hit, err := (publicsvc.Service{Redis: cache}).PublicSettings(ctx, permission.GuestActor())
		if err != nil || !reflect.DeepEqual(hit, want) {
			t.Fatalf("subsequent cache hit=%#v err=%v; want %#v", hit, err, want)
		}
	}
}

type publicCacheInvalidationFailure struct {
	redisstore.Store
	err   error
	calls int
}

func (s *publicCacheInvalidationFailure) InvalidatePublicSettings(context.Context) error {
	s.calls++
	return s.err
}
