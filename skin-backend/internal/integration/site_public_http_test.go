package integration_test

import (
	"context"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"testing"
)

func TestPublicSettingsAndAuthlibHeader(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	if err := db.Settings.Set(context.Background(), "site_name", "Public Name"); err != nil {
		t.Fatal(err)
	}
	resp := doJSON(t, h, "GET", "/v2/public/settings", nil)
	if resp.Code != 200 {
		t.Fatalf("public settings status=%d body=%s", resp.Code, resp.Body.String())
	}
	data := parseJSON(t, resp)
	if data["site_name"] != "Public Name" || data["allow_register"] != true || data["api_url"] != "http://localhost:8000" {
		t.Fatalf("unexpected public settings: %#v", data)
	}
	if got := resp.Result().Header.Get("X-Authlib-Injector-API-Location"); got != "http://localhost:8000" {
		t.Fatalf("missing authlib header: %q", got)
	}
}

func TestPublicSkinLibrarySearchAndWardrobeName(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	alice := testutil.CreateUser(t, db, "alice@test.com", "Password123", "ApiSearchAlice", false)
	bob := testutil.CreateUser(t, db, "bob@test.com", "Password123", "ApiSearchBob", false)
	charlie := testutil.CreateUser(t, db, "charlie@test.com", "Password123", "ApiSearchCharlie", false)
	if err := db.Textures.AddToLibrary(context.Background(), alice.ID, "aaaa", "skin", "MagicSword", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), bob.ID, "bbbb", "skin", "DragonShield", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), charlie.ID, "cccc", "skin", "HolyArmor", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), charlie.ID, "dddd", "cape", "SharedName", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), alice.ID, "eeee", "skin", "UniquePrivateTex", false, "default"); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{bob.ID, charlie.ID} {
		if ok, err := db.Textures.AddToWardrobe(context.Background(), userID, "aaaa", "skin"); err != nil || !ok {
			t.Fatalf("wardrobe add for most_used setup ok=%v err=%v", ok, err)
		}
	}
	if ok, err := db.Textures.AddToWardrobe(context.Background(), alice.ID, "bbbb", "skin"); err != nil || !ok {
		t.Fatalf("wardrobe add for second most_used setup ok=%v err=%v", ok, err)
	}

	resp := doJSON(t, h, "GET", "/v2/public/skin-library?q=MagicSword", nil)
	if resp.Code != 200 {
		t.Fatalf("library status=%d body=%s", resp.Code, resp.Body.String())
	}
	items := parseJSON(t, resp)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["hash"] != "aaaa" || items[0].(map[string]any)["uploader_name"] != "ApiSearchAlice" {
		t.Fatalf("unexpected name search items: %#v", items)
	}
	if byHash := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=bbb", nil))["items"].([]any); len(byHash) != 1 || byHash[0].(map[string]any)["hash"] != "bbbb" {
		t.Fatalf("hash search should return bob texture only: %#v", byHash)
	}
	if byUploader := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=ApiSearchCharlie", nil))["items"].([]any); len(byUploader) != 2 {
		t.Fatalf("uploader search should return both charlie textures: %#v", byUploader)
	}
	if lower := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=magicsword", nil))["items"].([]any); len(lower) != 1 || lower[0].(map[string]any)["hash"] != "aaaa" {
		t.Fatalf("search should be case-insensitive: %#v", lower)
	}
	if none := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=ZZZ_no_such_token", nil))["items"].([]any); len(none) != 0 {
		t.Fatalf("miss search should be empty: %#v", none)
	}
	if priv := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=UniquePrivateTex", nil))["items"].([]any); len(priv) != 0 {
		t.Fatalf("private matching texture should be excluded: %#v", priv)
	}
	mostUsed := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?sort=most_used&texture_type=skin&limit=2", nil))["items"].([]any)
	if len(mostUsed) != 2 || mostUsed[0].(map[string]any)["hash"] != "aaaa" || mostUsed[0].(map[string]any)["usage_count"] != float64(3) || mostUsed[1].(map[string]any)["hash"] != "bbbb" || mostUsed[1].(map[string]any)["usage_count"] != float64(2) {
		t.Fatalf("most_used sort should order by personal library user count: %#v", mostUsed)
	}
	if skins := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=SharedName&texture_type=skin", nil))["items"].([]any); len(skins) != 0 {
		t.Fatalf("skin filter should exclude matching cape: %#v", skins)
	}
	if capes := parseJSON(t, doJSON(t, h, "GET", "/v2/public/skin-library?q=SharedName&texture_type=cape", nil))["items"].([]any); len(capes) != 1 || capes[0].(map[string]any)["hash"] != "dddd" {
		t.Fatalf("cape filter should include matching cape only: %#v", capes)
	}
	for _, badLimit := range []string{"-1", "0", "99999999"} {
		clamped := doJSON(t, h, "GET", "/v2/public/skin-library?limit="+badLimit, nil)
		if clamped.Code != 200 {
			t.Fatalf("public library limit=%s should be clamped, got %d body=%s", badLimit, clamped.Code, clamped.Body.String())
		}
		items := parseJSON(t, clamped)["items"].([]any)
		if len(items) > util.MaxLimit {
			t.Fatalf("public library limit=%s returned too many items: %d", badLimit, len(items))
		}
	}

	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 10; i++ {
		path := "/v2/public/skin-library?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		page := parseJSON(t, doJSON(t, h, "GET", path, nil))
		for _, raw := range page["items"].([]any) {
			hash := raw.(map[string]any)["hash"].(string)
			if seen[hash] {
				t.Fatalf("public library pagination returned duplicate hash %q", hash)
			}
			seen[hash] = true
		}
		if page["has_next"] != true {
			break
		}
		cursor, _ = page["next_cursor"].(string)
		if cursor == "" {
			t.Fatalf("has_next page should include next_cursor: %#v", page)
		}
	}
	for _, hash := range []string{"aaaa", "bbbb", "cccc", "dddd"} {
		if !seen[hash] {
			t.Fatalf("public library pagination missed %s, saw %#v", hash, seen)
		}
	}
	if badCursor := doJSON(t, h, "GET", "/v2/public/skin-library?cursor=garbage!!", nil); badCursor.Code != 400 {
		t.Fatalf("invalid public library cursor should be 400, got %d body=%s", badCursor.Code, badCursor.Body.String())
	}
	if err := db.Settings.Set(context.Background(), "enable_skin_library", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	if disabled := doJSON(t, h, "GET", "/v2/public/skin-library", nil); disabled.Code != 403 {
		t.Fatalf("disabled public library should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
}
