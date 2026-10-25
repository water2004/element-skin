package settings_test

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestBooleanSettingsStoreCanonicalStringsAndReadExactBooleans(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: cache}
	for _, field := range booleanSettingFields {
		for _, enabled := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%t", field.key, enabled), func(t *testing.T) {
				if _, err := svc.GetGroup(t.Context(), field.group); err != nil {
					t.Fatal(err)
				}
				if err := svc.SaveGroupAndInvalidate(t.Context(), field.group, map[string]any{field.key: enabled}); err != nil {
					t.Fatal(err)
				}
				if raw, err := db.Settings.Get(t.Context(), field.key, ""); err != nil || raw != strconv.FormatBool(enabled) {
					t.Fatalf("stored %s=%q err=%v; want %q", field.key, raw, err, strconv.FormatBool(enabled))
				}
				if raw, err := cache.GetSetting(t.Context(), field.key); !errors.Is(err, redisstore.ErrCacheMiss) || raw != "" {
					t.Fatalf("saved setting retained cache=%q err=%v; want empty and ErrCacheMiss", raw, err)
				}
				group, err := svc.GetGroup(t.Context(), field.group)
				if err != nil || group[field.key] != enabled {
					t.Fatalf("management %s=%#v err=%v; want %t", field.key, group, err, enabled)
				}
				if got, err := svc.Bool(t.Context(), field.key, !enabled); err != nil || got != enabled {
					t.Fatalf("business %s=%t err=%v; want %t", field.key, got, err, enabled)
				}
			})
		}
	}
}

func TestStoredOneDoesNotEnableBooleanSettingsOrRewriteStorage(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: cache}
	for _, field := range booleanSettingFields {
		t.Run(field.key, func(t *testing.T) {
			if err := db.Settings.Set(t.Context(), field.key, "1"); err != nil {
				t.Fatal(err)
			}
			if err := svc.InvalidateCache(t.Context()); err != nil {
				t.Fatal(err)
			}
			group, err := svc.GetGroup(t.Context(), field.group)
			if err != nil || group[field.key] != false {
				t.Fatalf("management interpreted stored 1 as enabled: group=%#v err=%v", group, err)
			}
			if enabled, err := svc.Bool(t.Context(), field.key, true); err != nil || enabled {
				t.Fatalf("business interpreted stored 1 as enabled: value=%t err=%v", enabled, err)
			}
			if raw, err := db.Settings.Get(t.Context(), field.key, ""); err != nil || raw != "1" {
				t.Fatalf("read changed persisted value=%q err=%v; want unchanged 1", raw, err)
			}
			if raw, err := cache.GetSetting(t.Context(), field.key); err != nil || raw != "1" {
				t.Fatalf("raw cache=%q err=%v; want unchanged 1", raw, err)
			}
		})
	}
}

func TestInvalidBooleanSettingsPreserveEntireGroupAndCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: cache}
	public := map[string]any{"site_name": "Cached Site"}
	for _, field := range booleanSettingFields {
		if err := svc.SaveGroupAndInvalidate(t.Context(), field.group, map[string]any{field.key: true, field.companion: field.original}); err != nil {
			t.Fatal(err)
		}
		before, err := svc.GetGroup(t.Context(), field.group)
		if err != nil {
			t.Fatal(err)
		}
		if err := cache.SetPublicSettings(t.Context(), public, time.Minute); err != nil {
			t.Fatal(err)
		}
		for _, invalid := range invalidBooleanValues {
			t.Run(field.key+"/"+invalid.name, func(t *testing.T) {
				err := svc.SaveGroupAndInvalidate(t.Context(), field.group, map[string]any{field.key: invalid.value, field.companion: field.replacement})
				assertBooleanSettingError(t, err, field.key, 0)
				for key, want := range map[string]string{field.key: "true", field.companion: fmt.Sprint(field.original)} {
					if raw, err := db.Settings.Get(t.Context(), key, ""); err != nil || raw != want {
						t.Fatalf("rejection changed stored %s=%q err=%v; want %q", key, raw, err, want)
					}
					if raw, err := cache.GetSetting(t.Context(), key); err != nil || raw != want {
						t.Fatalf("rejection changed cached %s=%q err=%v; want %q", key, raw, err, want)
					}
				}
				if after, err := svc.GetGroup(t.Context(), field.group); err != nil || !reflect.DeepEqual(after, before) {
					t.Fatalf("rejection changed group=%#v err=%v; want %#v", after, err, before)
				}
				if after, err := cache.GetPublicSettings(t.Context()); err != nil || !reflect.DeepEqual(after, public) {
					t.Fatalf("rejection changed public cache=%#v err=%v; want %#v", after, err, public)
				}
			})
		}
	}
}

func TestInvalidFallbackBooleansPreserveEndpointsSettingsAndCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: cache}
	if err := svc.SaveGroupAndInvalidate(t.Context(), "fallback", map[string]any{"fallback_strategy": "serial", "fallbacks": []any{booleanFallbackEndpoint()}}); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetGroup(t.Context(), "fallback")
	if err != nil {
		t.Fatal(err)
	}
	public := map[string]any{"site_name": "Cached Site"}
	if err := cache.SetPublicSettings(t.Context(), public, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, key := range fallbackBooleanFields {
		for _, invalid := range invalidBooleanValues {
			t.Run(key+"/"+invalid.name, func(t *testing.T) {
				endpoint := booleanFallbackEndpoint()
				endpoint[key] = invalid.value
				err := svc.SaveGroupAndInvalidate(t.Context(), "fallback", map[string]any{"fallback_strategy": "parallel", "fallbacks": []any{endpoint}})
				assertBooleanSettingError(t, err, key, 1)
				if after, err := svc.GetGroup(t.Context(), "fallback"); err != nil || !reflect.DeepEqual(after, before) {
					t.Fatalf("rejection changed fallback group=%#v err=%v; want %#v", after, err, before)
				}
				if raw, err := db.Settings.Get(t.Context(), "fallback_strategy", ""); err != nil || raw != "serial" {
					t.Fatalf("rejection changed fallback strategy=%q err=%v; want serial", raw, err)
				}
				if raw, err := cache.GetSetting(t.Context(), "fallback_strategy"); err != nil || raw != "serial" {
					t.Fatalf("rejection changed cached fallback strategy=%q err=%v; want serial", raw, err)
				}
				if after, err := cache.GetPublicSettings(t.Context()); err != nil || !reflect.DeepEqual(after, public) {
					t.Fatalf("rejection changed public cache=%#v err=%v; want %#v", after, err, public)
				}
			})
		}
	}
}

func assertBooleanSettingError(t *testing.T, err error, key string, index int) {
	t.Helper()
	var apiErr util.HTTPError
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Error() != "setting_value.validate.invalid" {
		t.Fatalf("error=%#v; want exact 400 setting_value.validate.invalid", err)
	}
	want := map[string]any{"key": key, "expected_type": "boolean"}
	if index > 0 {
		want["index"] = index
	}
	if !reflect.DeepEqual(apiErr.Params, want) {
		t.Fatalf("error params=%#v; want %#v", apiErr.Params, want)
	}
}
