package settings_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsSaveGetAndPublicRoundTripExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	settings := settings.Settings{DB: db}

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
	if raw, _ := db.GetSetting(ctx, "unknown_should_skip", "missing"); raw != "missing" {
		t.Fatalf("unknown setting should not persist, got %q", raw)
	}

	if err := db.SetSetting(ctx, "smtp_password", "existing-secret"); err != nil {
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

	public, err := settings.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
	if err != nil {
		t.Fatal(err)
	}
	status := public["mojang_status_urls"].(map[string]any)
	if public["site_name"] != "Exact Skin" || public["allow_register"] != false ||
		public["site_url"] != "https://skin.example.com/root" || public["api_url"] != "https://api.example.com/skinapi" ||
		status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" {
		t.Fatalf("unexpected public settings: %#v", public)
	}
}

func TestSettingsRejectInvalidGroupAndProfileMode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db}
	if _, err := settings.GetGroup(context.Background(), "missing"); err == nil {
		t.Fatal("missing settings group should reject")
	}
	if err := settings.SaveGroup(context.Background(), "site", map[string]any{"profile_uuid_mode": "bad"}); err == nil {
		t.Fatal("invalid profile_uuid_mode should reject")
	}
}

func TestSettingsPublicPropagatesDatabaseErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db}
	db.Close()
	if out, err := settings.Public(context.Background(), "http://cfg-site.local/", "http://cfg-api.local/"); err == nil || out != nil {
		t.Fatalf("closed database should fail instead of returning partial public settings: out=%#v err=%v", out, err)
	}
}
