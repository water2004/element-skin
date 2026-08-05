package integration_test

import (
	"context"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestSiteProfileTextureHTTPFlows(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, db, "siteflow@test.com", "Password123", "SiteFlow", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "site_avatar_hash_123", "skin", "site avatar", false, "default"); err != nil {
		t.Fatal(err)
	}

	updateMe := doJSON(t, h, "PATCH", "/v2/users/me", map[string]any{"display_name": "UpdatedDisplayName", "avatar_hash": "site_avatar_hash_123"}, cookie)
	if updateMe.Code != http.StatusNoContent || updateMe.Body.Len() != 0 {
		t.Fatalf("update me status=%d body=%s", updateMe.Code, updateMe.Body.String())
	}
	me := parseJSON(t, doJSON(t, h, "GET", "/v2/users/me", nil, cookie))
	if me["display_name"] != "UpdatedDisplayName" || me["avatar_hash"] != "site_avatar_hash_123" {
		t.Fatalf("update me did not persist: %#v", me)
	}

	if err := db.Settings.Set(context.Background(), "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	offline := doJSON(t, h, "POST", "/v2/users/me/profiles", map[string]any{"name": "OfflinePlayerA", "model": "default"}, cookie)
	if offline.Code != http.StatusCreated {
		t.Fatalf("offline profile status=%d body=%s", offline.Code, offline.Body.String())
	}
	if parseJSON(t, offline)["id"] != util.OfflineUUIDNoDash("OfflinePlayerA") {
		t.Fatalf("offline profile should use offline UUID: %s", offline.Body.String())
	}
	if err := db.Settings.Set(context.Background(), "profile_uuid_mode", "random"); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)

	create := doJSON(t, h, "POST", "/v2/users/me/profiles", map[string]any{"name": "ApiPlayer", "model": "default"}, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("create profile status=%d body=%s", create.Code, create.Body.String())
	}
	profileID := parseJSON(t, create)["id"].(string)
	for i := 0; i < 5; i++ {
		if err := db.Profiles.Create(context.Background(), model.Profile{ID: "http_profile_" + strconv.Itoa(i), UserID: user.ID, Name: "HTTPProfile_" + strconv.Itoa(i), TextureModel: "default"}); err != nil {
			t.Fatal(err)
		}
	}
	seenProfiles := map[string]bool{}
	profileCursor := ""
	for i := 0; i < 20; i++ {
		path := "/v2/users/me/profiles?limit=2"
		if profileCursor != "" {
			path += "&cursor=" + url.QueryEscape(profileCursor)
		}
		pageResp := doJSON(t, h, "GET", path, nil, cookie)
		if pageResp.Code != 200 {
			t.Fatalf("me profiles page status=%d body=%s", pageResp.Code, pageResp.Body.String())
		}
		page := parseJSON(t, pageResp)
		for _, raw := range page["items"].([]any) {
			id := raw.(map[string]any)["id"].(string)
			if seenProfiles[id] {
				t.Fatalf("duplicate /v2/users/me/profiles item %q", id)
			}
			seenProfiles[id] = true
		}
		if page["has_next"] != true {
			break
		}
		profileCursor = page["next_cursor"].(string)
		if profileCursor == "" {
			t.Fatalf("has_next /v2/users/me/profiles response should include next_cursor: %#v", page)
		}
	}
	for i := 0; i < 5; i++ {
		id := "http_profile_" + strconv.Itoa(i)
		if !seenProfiles[id] {
			t.Fatalf("/v2/users/me/profiles pagination missed %s, saw %#v", id, seenProfiles)
		}
	}

	rename := doJSON(t, h, "PATCH", "/v2/users/me/profiles/"+profileID, map[string]any{"name": "NewFancyName"}, cookie)
	if rename.Code != http.StatusNoContent || rename.Body.Len() != 0 {
		t.Fatalf("rename status=%d body=%s", rename.Code, rename.Body.String())
	}
	p, _ := db.Profiles.GetByID(context.Background(), profileID)
	if p.Name != "NewFancyName" {
		t.Fatalf("profile not renamed: %#v", p)
	}

	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "apply_hash", "skin", "ApplySkin", false, "slim"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := db.Textures.AddToLibrary(context.Background(), user.ID, "http_tex_"+strconv.Itoa(i), "skin", "HTTP Texture "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	expectedTextures := map[string]bool{"apply_hash": true, "site_avatar_hash_123": true}
	for i := 0; i < 3; i++ {
		expectedTextures["http_tex_"+strconv.Itoa(i)] = true
	}
	seenTextures := map[string]bool{}
	textureCursor := ""
	for i := 0; i < 20; i++ {
		path := "/v2/users/me/textures?limit=2"
		if textureCursor != "" {
			path += "&cursor=" + url.QueryEscape(textureCursor)
		}
		pageResp := doJSON(t, h, "GET", path, nil, cookie)
		if pageResp.Code != 200 {
			t.Fatalf("me textures page status=%d body=%s", pageResp.Code, pageResp.Body.String())
		}
		page := parseJSON(t, pageResp)
		for _, raw := range page["items"].([]any) {
			item := raw.(map[string]any)
			hash := item["hash"].(string)
			if !expectedTextures[hash] {
				t.Fatalf("/v2/users/me/textures returned unexpected hash %q in item %#v; expected one of %#v", hash, item, expectedTextures)
			}
			if seenTextures[hash] {
				t.Fatalf("duplicate /v2/users/me/textures item %q", hash)
			}
			seenTextures[hash] = true
		}
		if page["has_next"] != true {
			break
		}
		textureCursor = page["next_cursor"].(string)
		if textureCursor == "" {
			t.Fatalf("has_next /v2/users/me/textures response should include next_cursor: %#v", page)
		}
	}
	if len(seenTextures) != len(expectedTextures) {
		t.Fatalf("/v2/users/me/textures pagination saw %d textures, want %d: saw=%#v want=%#v", len(seenTextures), len(expectedTextures), seenTextures, expectedTextures)
	}
	for hash := range expectedTextures {
		if !seenTextures[hash] {
			t.Fatalf("/v2/users/me/textures pagination missed %s, saw %#v", hash, seenTextures)
		}
	}
	for _, badLimit := range []string{"-1", "0", "99999999"} {
		clamped := doJSON(t, h, "GET", "/v2/users/me/textures?limit="+badLimit, nil, cookie)
		if clamped.Code != 200 {
			t.Fatalf("/v2/users/me/textures limit=%s should be clamped, got %d body=%s", badLimit, clamped.Code, clamped.Body.String())
		}
		items := parseJSON(t, clamped)["items"].([]any)
		if len(items) > util.MaxLimit {
			t.Fatalf("/v2/users/me/textures limit=%s returned too many items: %d", badLimit, len(items))
		}
	}

	libraryOwner := testutil.CreateUser(t, db, "library-owner@test.com", "Password123", "LibraryOwner", false)
	if err := db.Textures.AddToLibrary(context.Background(), libraryOwner.ID, "lib_tex_hash_123", "skin", "Epic Skin Name", true, "default"); err != nil {
		t.Fatal(err)
	}
	addMissing := doJSON(t, h, "POST", "/v2/users/me/textures/nonexistent_hash/wardrobe", nil, cookie)
	if addMissing.Code != 404 {
		t.Fatalf("adding missing library texture should be 404, got %d body=%s", addMissing.Code, addMissing.Body.String())
	}
	addLibrary := doJSON(t, h, "POST", "/v2/users/me/textures/lib_tex_hash_123/wardrobe", nil, cookie)
	if addLibrary.Code != http.StatusNoContent || addLibrary.Body.Len() != 0 {
		t.Fatalf("add library texture status=%d body=%s", addLibrary.Code, addLibrary.Body.String())
	}
	addedInfo, _ := db.Textures.GetInfo(context.Background(), user.ID, "lib_tex_hash_123", "skin")
	if addedInfo == nil || addedInfo["note"] != "Epic Skin Name" {
		t.Fatalf("added library texture should preserve name: %#v", addedInfo)
	}
	missingDetail := doJSON(t, h, "GET", "/v2/users/me/textures/nope/skin", nil, cookie)
	if missingDetail.Code != 404 {
		t.Fatalf("missing texture detail should be 404, got %d body=%s", missingDetail.Code, missingDetail.Body.String())
	}

	apply := doJSON(t, h, "POST", "/v2/users/me/textures/apply_hash/apply", map[string]any{"profile_id": profileID, "texture_type": "skin"}, cookie)
	if apply.Code != http.StatusNoContent || apply.Body.Len() != 0 {
		t.Fatalf("apply status=%d body=%s", apply.Code, apply.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p.SkinHash == nil || *p.SkinHash != "apply_hash" || p.TextureModel != "slim" {
		t.Fatalf("texture not applied: %#v", p)
	}

	detail := doJSON(t, h, "GET", "/v2/users/me/textures/apply_hash/skin", nil, cookie)
	if detail.Code != 200 {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	if parseJSON(t, detail)["note"] != "ApplySkin" {
		t.Fatalf("unexpected detail: %s", detail.Body.String())
	}

	update := doJSON(t, h, "PATCH", "/v2/users/me/textures/apply_hash/skin", map[string]any{"note": "RenamedSkin", "is_public": true}, cookie)
	if update.Code != 200 {
		t.Fatalf("update texture status=%d body=%s", update.Code, update.Body.String())
	}
	info, _ := db.Textures.GetInfo(context.Background(), user.ID, "apply_hash", "skin")
	if info["note"] != "RenamedSkin" || info["is_public"].(int) != 1 {
		t.Fatalf("texture update did not persist: %#v", info)
	}

	clear := doJSON(t, h, "DELETE", "/v2/users/me/profiles/"+profileID+"/skin", nil, cookie)
	if clear.Code != http.StatusNoContent || clear.Body.Len() != 0 {
		t.Fatalf("clear status=%d body=%s", clear.Code, clear.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p.SkinHash != nil {
		t.Fatalf("skin should be cleared: %#v", p)
	}

	del := doJSON(t, h, "DELETE", "/v2/users/me/profiles/"+profileID, nil, cookie)
	if del.Code != http.StatusNoContent || del.Body.Len() != 0 {
		t.Fatalf("delete profile status=%d body=%s", del.Code, del.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p != nil {
		t.Fatal("profile should be deleted")
	}
}
