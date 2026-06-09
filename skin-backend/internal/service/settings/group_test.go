package settings_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
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
