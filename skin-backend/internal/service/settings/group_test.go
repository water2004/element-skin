package settings_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSettingsSaveGetRoundTripExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}

	if err := settings.SaveGroup(ctx, "site", map[string]any{
		"site_name":           "Exact Skin",
		"allow_register":      false,
		"max_texture_size":    2048,
		"profile_uuid_mode":   "offline",
		"site_url":            "skin.example.com/root/",
		"api_url":             "https://api.example.com/skinapi/",
		"unknown_should_skip": "x",
	}); err != nil {
		t.Fatal(err)
	}
	site, err := settings.GetGroup(ctx, "site")
	if err != nil {
		t.Fatal(err)
	}
	if site["site_name"] != "Exact Skin" || site["allow_register"] != false || site["max_texture_size"] != 2048 ||
		site["profile_uuid_mode"] != "offline" || site["site_url"] != "skin.example.com/root/" || site["api_url"] != "https://api.example.com/skinapi/" {
		t.Fatalf("unexpected site settings: %#v", site)
	}
	if raw, _ := db.Settings.Get(ctx, "unknown_should_skip", "missing"); raw != "missing" {
		t.Fatalf("unknown setting should not persist, got %q", raw)
	}

	if err := db.Settings.Set(ctx, "smtp_password", "existing-secret"); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "email", map[string]any{"smtp_host": "mail.example.com", "smtp_password": ""}); err != nil {
		t.Fatal(err)
	}
	email, err := settings.GetGroup(ctx, "email")
	if err != nil {
		t.Fatal(err)
	}
	if email["smtp_host"] != "mail.example.com" || email["smtp_password"] != "existing-secret" || email["smtp_ssl"] != true || email["smtp_port"] != 465 {
		t.Fatalf("unexpected email settings: %#v", email)
	}

	if err := settings.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"priority":         1,
			"session_url":      "https://session.example",
			"account_url":      "https://account.example",
			"services_url":     "https://services.example",
			"cache_ttl":        30,
			"skin_domains":     "skins.example, cdn.example",
			"enable_profile":   true,
			"enable_hasjoined": false,
			"enable_whitelist": true,
			"note":             "primary",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	fallback, err := settings.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if fallback["fallback_strategy"] != "parallel" {
		t.Fatalf("fallback strategy did not persist: %#v", fallback)
	}
	fallbacks := fallback["fallbacks"].([]map[string]any)
	if len(fallbacks) != 1 || fallbacks[0]["session_url"] != "https://session.example" || fallbacks[0]["enable_hasjoined"] != false {
		t.Fatalf("unexpected fallback endpoints: %#v", fallbacks)
	}

	if err := settings.SaveGroup(ctx, "easter_eggs", map[string]any{
		"easter_eggs_enabled": []any{"april-fools", "christmas", "dragon-boat", "april-fools"},
	}); err != nil {
		t.Fatal(err)
	}
	easterEggs, err := settings.GetGroup(ctx, "easter_eggs")
	if err != nil {
		t.Fatal(err)
	}
	enabled := easterEggs["easter_eggs_enabled"].([]string)
	if len(enabled) != 3 || enabled[0] != "april-fools" || enabled[1] != "christmas" || enabled[2] != "dragon-boat" {
		t.Fatalf("unexpected easter egg settings: %#v", easterEggs)
	}
}

func TestSettingsRejectInvalidGroupAndProfileMode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if _, err := settings.GetGroup(context.Background(), "missing"); err == nil {
		t.Fatal("missing settings group should reject")
	}
	if err := settings.SaveGroup(context.Background(), "site", map[string]any{"profile_uuid_mode": "bad"}); err == nil {
		t.Fatal("invalid profile_uuid_mode should reject")
	}
	if err := settings.SaveGroup(context.Background(), "easter_eggs", map[string]any{"easter_eggs_enabled": []any{"missing"}}); err == nil {
		t.Fatal("invalid easter egg should reject")
	}
}

func TestSettingsInvalidFallbackGroupPreservesExistingConfiguration(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "serial",
		"fallbacks": []any{map[string]any{
			"priority":         7,
			"session_url":      "https://old-session.example",
			"account_url":      "https://old-account.example",
			"services_url":     "https://old-services.example",
			"cache_ttl":        45,
			"skin_domains":     "old-skins.example",
			"enable_profile":   false,
			"enable_hasjoined": true,
			"enable_whitelist": true,
			"note":             "existing",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"session_url":  "https://new-session.example",
			"account_url":  "",
			"services_url": "https://new-services.example",
		}},
	})
	httpErr, ok := err.(util.HTTPError)
	if !ok || httpErr.Status != 400 || httpErr.Detail != "fallback[1] urls are required" {
		t.Fatalf("invalid fallback error = %#v, want exact 400 validation error", err)
	}

	strategy, err := db.Settings.Get(ctx, "fallback_strategy", "")
	if err != nil || strategy != "serial" {
		t.Fatalf("failed fallback save changed strategy: strategy=%q err=%v", strategy, err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("failed fallback save changed endpoint count: %#v", endpoints)
	}
	got := endpoints[0]
	if got["priority"] != 7 ||
		got["session_url"] != "https://old-session.example" ||
		got["account_url"] != "https://old-account.example" ||
		got["services_url"] != "https://old-services.example" ||
		got["cache_ttl"] != 45 ||
		got["skin_domains"] != "old-skins.example" ||
		got["enable_profile"] != false ||
		got["enable_hasjoined"] != true ||
		got["enable_whitelist"] != true ||
		got["note"] != "existing" {
		t.Fatalf("failed fallback save changed existing endpoint: %#v", got)
	}
}

func TestSettingsFallbackDatabaseFailureRollsBackStrategyAndEndpoints(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "serial",
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://stable-session.example",
			"account_url":  "https://stable-account.example",
			"services_url": "https://stable-services.example",
			"cache_ttl":    60,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE fallback_endpoints ADD CONSTRAINT reject_parallel_endpoint CHECK (note <> 'reject')`,
	); err != nil {
		t.Fatal(err)
	}

	err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"priority":     2,
			"session_url":  "https://rejected-session.example",
			"account_url":  "https://rejected-account.example",
			"services_url": "https://rejected-services.example",
			"cache_ttl":    30,
			"note":         "reject",
		}},
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("fallback database failure = %#v, want PostgreSQL 23514", err)
	}
	strategy, err := db.Settings.Get(ctx, "fallback_strategy", "")
	if err != nil || strategy != "serial" {
		t.Fatalf("failed transaction changed strategy: strategy=%q err=%v", strategy, err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 ||
		endpoints[0]["priority"] != 1 ||
		endpoints[0]["session_url"] != "https://stable-session.example" ||
		endpoints[0]["account_url"] != "https://stable-account.example" ||
		endpoints[0]["services_url"] != "https://stable-services.example" ||
		endpoints[0]["cache_ttl"] != 60 {
		t.Fatalf("failed transaction changed endpoints: %#v", endpoints)
	}
}

func TestSettingsFallbackProbeIntervalValidationAndPersistence(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	redis := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: redis}

	defaults, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if defaults["fallback_probe_interval"] != 600 {
		t.Fatalf("default probe interval should be 600 seconds, got %#v", defaults["fallback_probe_interval"])
	}

	if err := svc.SaveGroup(ctx, "fallback", map[string]any{"fallback_probe_interval": 1800}); err != nil {
		t.Fatalf("valid probe interval should persist: %v", err)
	}
	if err := redis.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if updated["fallback_probe_interval"] != 1800 {
		t.Fatalf("probe interval did not persist: %#v", updated["fallback_probe_interval"])
	}

	cases := []any{59, 86401, "abc", -10}
	for _, value := range cases {
		err := svc.SaveGroup(ctx, "fallback", map[string]any{"fallback_probe_interval": value})
		httpErr, ok := err.(util.HTTPError)
		if !ok || httpErr.Status != 400 {
			t.Fatalf("invalid probe interval %v should return HTTP 400, got %#v", value, err)
		}
	}
	if err := redis.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	preserved, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if preserved["fallback_probe_interval"] != 1800 {
		t.Fatalf("invalid attempts changed persisted interval: %#v", preserved["fallback_probe_interval"])
	}
}
