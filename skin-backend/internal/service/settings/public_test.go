package settings_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsPublicUsesSavedValuesAndPrimaryFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := settings.SaveGroup(ctx, "site", map[string]any{
		"site_name":      "Exact Skin",
		"allow_register": false,
		"site_url":       "skin.example.com/root/",
		"api_url":        "https://api.example.com/skinapi/",
	}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://session.example",
			"account_url":  "https://account.example",
			"services_url": "https://services.example",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "easter_eggs", map[string]any{
		"easter_eggs_enabled": []any{"april-fools", "children-day", "mid-autumn"},
	}); err != nil {
		t.Fatal(err)
	}

	public, err := settings.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
	if err != nil {
		t.Fatal(err)
	}
	status := public["mojang_status_urls"].(map[string]any)
	easterEggs := public["easter_eggs"].(map[string]any)
	enabled := easterEggs["enabled"].([]string)
	if public["site_name"] != "Exact Skin" || public["allow_register"] != false ||
		public["site_url"] != "https://skin.example.com/root" || public["api_url"] != "https://api.example.com/skinapi" ||
		status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" ||
		len(enabled) != 3 || enabled[0] != "april-fools" || enabled[1] != "children-day" || enabled[2] != "mid-autumn" {
		t.Fatalf("unexpected public settings: %#v", public)
	}
}

func TestSettingsPublicPropagatesDatabaseErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	db.Close()
	if out, err := settings.Public(context.Background(), "http://cfg-site.local/", "http://cfg-api.local/"); err == nil || out != nil {
		t.Fatalf("closed database should fail instead of returning partial public settings: out=%#v err=%v", out, err)
	}
}
