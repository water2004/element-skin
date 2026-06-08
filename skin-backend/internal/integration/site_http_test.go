package integration_test

import (
	"context"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSiteLoginMeAndRefresh(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "api_login@test.com", "ApiPassword123", "LoginUser", false)

	login := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	if login.Code != 200 {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	body := parseJSON(t, login)
	if body["user_id"] != user.ID || body["is_admin"] != false {
		t.Fatalf("unexpected login body: %#v", body)
	}
	access := cookieNamed(login, "access_token")
	refresh := cookieNamed(login, "refresh_token")
	if access == nil || refresh == nil {
		t.Fatalf("missing session cookies: %#v", login.Result().Cookies())
	}

	me := doJSON(t, h, "GET", "/me", nil, access)
	if me.Code != 200 {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
	meBody := parseJSON(t, me)
	if meBody["id"] != user.ID {
		t.Fatalf("unexpected me body: %#v", meBody)
	}
	if _, ok := meBody["profiles"]; ok {
		t.Fatalf("/me should not inline profiles: %#v", meBody)
	}
	if meBody["profile_count"] != float64(0) || meBody["texture_count"] != float64(0) {
		t.Fatalf("/me counts should start at zero: %#v", meBody)
	}
	if err := db.Users.Ban(context.Background(), user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	bannedMe := doJSON(t, h, "GET", "/me", nil, access)
	if bannedMe.Code != 200 {
		t.Fatalf("banned user should still access site API, got %d body=%s", bannedMe.Code, bannedMe.Body.String())
	}
	if err := db.Users.Unban(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}

	rotated := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
	if rotated.Code != 200 {
		t.Fatalf("refresh status=%d body=%s", rotated.Code, rotated.Body.String())
	}
	newRefresh := cookieNamed(rotated, "refresh_token")
	if newRefresh == nil || newRefresh.Value == refresh.Value {
		t.Fatal("refresh token was not rotated")
	}
	replay := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
	if replay.Code != 401 {
		t.Fatalf("old refresh should be rejected, got %d", replay.Code)
	}
	missingRefresh := doJSON(t, h, "POST", "/me/refresh-token", nil)
	if missingRefresh.Code != 401 {
		t.Fatalf("missing refresh should be 401, got %d", missingRefresh.Code)
	}

	noAccessLogin := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	noAccessRefresh := cookieNamed(noAccessLogin, "refresh_token")
	expiredAccess, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	meExpired := doJSON(t, h, "GET", "/me", nil, &http.Cookie{Name: "access_token", Value: expiredAccess})
	if meExpired.Code != 401 {
		t.Fatalf("expired access should be rejected, got %d", meExpired.Code)
	}
	refreshWithoutAccess := doJSON(t, h, "POST", "/me/refresh-token", nil, noAccessRefresh)
	if refreshWithoutAccess.Code != 200 {
		t.Fatalf("refresh should work without valid access, got %d body=%s", refreshWithoutAccess.Code, refreshWithoutAccess.Body.String())
	}

	logoutLogin := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	logoutRefresh := cookieNamed(logoutLogin, "refresh_token")
	logout := doJSON(t, h, "POST", "/site-logout", nil, logoutRefresh)
	if logout.Code != 200 {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}
	afterLogout := doJSON(t, h, "POST", "/me/refresh-token", nil, logoutRefresh)
	if afterLogout.Code != 401 {
		t.Fatalf("refresh after logout should be 401, got %d", afterLogout.Code)
	}

	chpwLogin := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	chpwAccess := cookieNamed(chpwLogin, "access_token")
	chpwRefresh := cookieNamed(chpwLogin, "refresh_token")
	chpw := doJSON(t, h, "POST", "/me/password", map[string]any{"old_password": "ApiPassword123", "new_password": "NewPassword456!"}, chpwAccess)
	if chpw.Code != 200 {
		t.Fatalf("change password status=%d body=%s", chpw.Code, chpw.Body.String())
	}
	afterPasswordChange := doJSON(t, h, "POST", "/me/refresh-token", nil, chpwRefresh)
	if afterPasswordChange.Code != 401 {
		t.Fatalf("refresh after password change should be 401, got %d", afterPasswordChange.Code)
	}

	deletedUser := testutil.CreateUser(t, db, "refresh_deleted@test.com", "Password123", "RefreshDeleted", false)
	deletedLogin := doJSON(t, h, "POST", "/site-login", map[string]any{"email": deletedUser.Email, "password": "Password123"})
	deletedRefresh := cookieNamed(deletedLogin, "refresh_token")
	if ok, err := db.Users.Delete(context.Background(), deletedUser.ID); err != nil || !ok {
		t.Fatalf("delete refresh test user ok=%v err=%v", ok, err)
	}
	afterDelete := doJSON(t, h, "POST", "/me/refresh-token", nil, deletedRefresh)
	if afterDelete.Code != 401 {
		t.Fatalf("refresh after user deletion should be 401, got %d", afterDelete.Code)
	}
	deletedAccess, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, deletedUser.ID, false, time.Hour)
	deletedMe := doJSON(t, h, "GET", "/me", nil, &http.Cookie{Name: "access_token", Value: deletedAccess})
	if deletedMe.Code != 401 {
		t.Fatalf("access token for deleted user should be rejected, got %d", deletedMe.Code)
	}
}

func TestPublicSettingsAndAuthlibHeader(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	if err := db.Settings.Set(context.Background(), "site_name", "Public Name"); err != nil {
		t.Fatal(err)
	}
	resp := doJSON(t, h, "GET", "/public/settings", nil)
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

	resp := doJSON(t, h, "GET", "/public/skin-library?q=MagicSword", nil)
	if resp.Code != 200 {
		t.Fatalf("library status=%d body=%s", resp.Code, resp.Body.String())
	}
	items := parseJSON(t, resp)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["hash"] != "aaaa" || items[0].(map[string]any)["uploader_name"] != "ApiSearchAlice" {
		t.Fatalf("unexpected name search items: %#v", items)
	}
	if byHash := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=bbb", nil))["items"].([]any); len(byHash) != 1 || byHash[0].(map[string]any)["hash"] != "bbbb" {
		t.Fatalf("hash search should return bob texture only: %#v", byHash)
	}
	if byUploader := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=ApiSearchCharlie", nil))["items"].([]any); len(byUploader) != 2 {
		t.Fatalf("uploader search should return both charlie textures: %#v", byUploader)
	}
	if lower := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=magicsword", nil))["items"].([]any); len(lower) != 1 || lower[0].(map[string]any)["hash"] != "aaaa" {
		t.Fatalf("search should be case-insensitive: %#v", lower)
	}
	if none := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=ZZZ_no_such_token", nil))["items"].([]any); len(none) != 0 {
		t.Fatalf("miss search should be empty: %#v", none)
	}
	if priv := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=UniquePrivateTex", nil))["items"].([]any); len(priv) != 0 {
		t.Fatalf("private matching texture should be excluded: %#v", priv)
	}
	mostUsed := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?sort=most_used&texture_type=skin&limit=2", nil))["items"].([]any)
	if len(mostUsed) != 2 || mostUsed[0].(map[string]any)["hash"] != "aaaa" || mostUsed[0].(map[string]any)["usage_count"] != float64(3) || mostUsed[1].(map[string]any)["hash"] != "bbbb" || mostUsed[1].(map[string]any)["usage_count"] != float64(2) {
		t.Fatalf("most_used sort should order by personal library user count: %#v", mostUsed)
	}
	if skins := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=SharedName&texture_type=skin", nil))["items"].([]any); len(skins) != 0 {
		t.Fatalf("skin filter should exclude matching cape: %#v", skins)
	}
	if capes := parseJSON(t, doJSON(t, h, "GET", "/public/skin-library?q=SharedName&texture_type=cape", nil))["items"].([]any); len(capes) != 1 || capes[0].(map[string]any)["hash"] != "dddd" {
		t.Fatalf("cape filter should include matching cape only: %#v", capes)
	}
	for _, badLimit := range []string{"-1", "0", "99999999"} {
		clamped := doJSON(t, h, "GET", "/public/skin-library?limit="+badLimit, nil)
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
		path := "/public/skin-library?limit=2"
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
	if badCursor := doJSON(t, h, "GET", "/public/skin-library?cursor=garbage!!", nil); badCursor.Code != 400 {
		t.Fatalf("invalid public library cursor should be 400, got %d body=%s", badCursor.Code, badCursor.Body.String())
	}
	if err := db.Settings.Set(context.Background(), "enable_skin_library", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	if disabled := doJSON(t, h, "GET", "/public/skin-library", nil); disabled.Code != 403 {
		t.Fatalf("disabled public library should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
}

func TestSiteProfileTextureHTTPFlows(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, db, "siteflow@test.com", "Password123", "SiteFlow", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}

	updateMe := doJSON(t, h, "PATCH", "/me", map[string]any{"display_name": "UpdatedDisplayName", "avatar_hash": "fake_avatar_hash_123"}, cookie)
	if updateMe.Code != 200 {
		t.Fatalf("update me status=%d body=%s", updateMe.Code, updateMe.Body.String())
	}
	me := parseJSON(t, doJSON(t, h, "GET", "/me", nil, cookie))
	if me["display_name"] != "UpdatedDisplayName" || me["avatar_hash"] != "fake_avatar_hash_123" {
		t.Fatalf("update me did not persist: %#v", me)
	}

	if err := db.Settings.Set(context.Background(), "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	offline := doJSON(t, h, "POST", "/me/profiles", map[string]any{"name": "OfflinePlayerA", "model": "default"}, cookie)
	if offline.Code != 200 {
		t.Fatalf("offline profile status=%d body=%s", offline.Code, offline.Body.String())
	}
	if parseJSON(t, offline)["id"] != util.OfflineUUIDNoDash("OfflinePlayerA") {
		t.Fatalf("offline profile should use offline UUID: %s", offline.Body.String())
	}
	if err := db.Settings.Set(context.Background(), "profile_uuid_mode", "random"); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)

	create := doJSON(t, h, "POST", "/me/profiles", map[string]any{"name": "ApiPlayer", "model": "default"}, cookie)
	if create.Code != 200 {
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
		path := "/me/profiles?limit=2"
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
				t.Fatalf("duplicate /me/profiles item %q", id)
			}
			seenProfiles[id] = true
		}
		if page["has_next"] != true {
			break
		}
		profileCursor = page["next_cursor"].(string)
		if profileCursor == "" {
			t.Fatalf("has_next /me/profiles response should include next_cursor: %#v", page)
		}
	}
	for i := 0; i < 5; i++ {
		id := "http_profile_" + strconv.Itoa(i)
		if !seenProfiles[id] {
			t.Fatalf("/me/profiles pagination missed %s, saw %#v", id, seenProfiles)
		}
	}

	rename := doJSON(t, h, "PATCH", "/me/profiles/"+profileID, map[string]any{"name": "NewFancyName"}, cookie)
	if rename.Code != 200 {
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
	expectedTextures := map[string]bool{"apply_hash": true}
	for i := 0; i < 3; i++ {
		expectedTextures["http_tex_"+strconv.Itoa(i)] = true
	}
	seenTextures := map[string]bool{}
	textureCursor := ""
	for i := 0; i < 20; i++ {
		path := "/me/textures?limit=2"
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
				t.Fatalf("/me/textures returned unexpected hash %q in item %#v; expected one of %#v", hash, item, expectedTextures)
			}
			if seenTextures[hash] {
				t.Fatalf("duplicate /me/textures item %q", hash)
			}
			seenTextures[hash] = true
		}
		if page["has_next"] != true {
			break
		}
		textureCursor = page["next_cursor"].(string)
		if textureCursor == "" {
			t.Fatalf("has_next /me/textures response should include next_cursor: %#v", page)
		}
	}
	if len(seenTextures) != len(expectedTextures) {
		t.Fatalf("/me/textures pagination saw %d textures, want %d: saw=%#v want=%#v", len(seenTextures), len(expectedTextures), seenTextures, expectedTextures)
	}
	for hash := range expectedTextures {
		if !seenTextures[hash] {
			t.Fatalf("/me/textures pagination missed %s, saw %#v", hash, seenTextures)
		}
	}
	for _, badLimit := range []string{"-1", "0", "99999999"} {
		clamped := doJSON(t, h, "GET", "/me/textures?limit="+badLimit, nil, cookie)
		if clamped.Code != 200 {
			t.Fatalf("/me/textures limit=%s should be clamped, got %d body=%s", badLimit, clamped.Code, clamped.Body.String())
		}
		items := parseJSON(t, clamped)["items"].([]any)
		if len(items) > util.MaxLimit {
			t.Fatalf("/me/textures limit=%s returned too many items: %d", badLimit, len(items))
		}
	}

	libraryOwner := testutil.CreateUser(t, db, "library-owner@test.com", "Password123", "LibraryOwner", false)
	if err := db.Textures.AddToLibrary(context.Background(), libraryOwner.ID, "lib_tex_hash_123", "skin", "Epic Skin Name", true, "default"); err != nil {
		t.Fatal(err)
	}
	addMissing := doJSON(t, h, "POST", "/me/textures/nonexistent_hash/add", nil, cookie)
	if addMissing.Code != 404 {
		t.Fatalf("adding missing library texture should be 404, got %d body=%s", addMissing.Code, addMissing.Body.String())
	}
	addLibrary := doJSON(t, h, "POST", "/me/textures/lib_tex_hash_123/add", nil, cookie)
	if addLibrary.Code != 200 {
		t.Fatalf("add library texture status=%d body=%s", addLibrary.Code, addLibrary.Body.String())
	}
	addedInfo, _ := db.Textures.GetInfo(context.Background(), user.ID, "lib_tex_hash_123", "skin")
	if addedInfo == nil || addedInfo["note"] != "Epic Skin Name" {
		t.Fatalf("added library texture should preserve name: %#v", addedInfo)
	}
	missingDetail := doJSON(t, h, "GET", "/me/textures/nope/skin", nil, cookie)
	if missingDetail.Code != 404 {
		t.Fatalf("missing texture detail should be 404, got %d body=%s", missingDetail.Code, missingDetail.Body.String())
	}

	apply := doJSON(t, h, "POST", "/me/textures/apply_hash/apply", map[string]any{"profile_id": profileID, "texture_type": "skin"}, cookie)
	if apply.Code != 200 {
		t.Fatalf("apply status=%d body=%s", apply.Code, apply.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p.SkinHash == nil || *p.SkinHash != "apply_hash" || p.TextureModel != "slim" {
		t.Fatalf("texture not applied: %#v", p)
	}

	detail := doJSON(t, h, "GET", "/me/textures/apply_hash/skin", nil, cookie)
	if detail.Code != 200 {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	if parseJSON(t, detail)["note"] != "ApplySkin" {
		t.Fatalf("unexpected detail: %s", detail.Body.String())
	}

	update := doJSON(t, h, "PATCH", "/me/textures/apply_hash/skin", map[string]any{"note": "RenamedSkin", "is_public": true}, cookie)
	if update.Code != 200 {
		t.Fatalf("update texture status=%d body=%s", update.Code, update.Body.String())
	}
	info, _ := db.Textures.GetInfo(context.Background(), user.ID, "apply_hash", "skin")
	if info["note"] != "RenamedSkin" || info["is_public"].(int) != 1 {
		t.Fatalf("texture update did not persist: %#v", info)
	}

	clear := doJSON(t, h, "DELETE", "/me/profiles/"+profileID+"/skin", nil, cookie)
	if clear.Code != 200 {
		t.Fatalf("clear status=%d body=%s", clear.Code, clear.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p.SkinHash != nil {
		t.Fatalf("skin should be cleared: %#v", p)
	}

	del := doJSON(t, h, "DELETE", "/me/profiles/"+profileID, nil, cookie)
	if del.Code != 200 {
		t.Fatalf("delete profile status=%d body=%s", del.Code, del.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profileID)
	if p != nil {
		t.Fatal("profile should be deleted")
	}
}

func TestSelfDeleteAndDirectTextureUploadHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "selfdelete@test.com", "Password123", "SelfDelete", false)
	profile := testutil.CreateProfile(t, db, user.ID, "direct_upload_profile", "DirectUpload")
	access, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: access}

	direct := doMultipart(t, h, "POST", "/textures/upload", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "skin",
		"model":        "slim",
		"is_public":    "false",
	}, "file", "skin.png", pngTexture(t, 64, 64), cookie)
	if direct.Code != 200 {
		t.Fatalf("direct upload status=%d body=%s", direct.Code, direct.Body.String())
	}
	updated, _ := db.Profiles.GetByID(context.Background(), profile.ID)
	if updated.SkinHash == nil || updated.TextureModel != "slim" {
		t.Fatalf("direct upload did not apply texture/model: %#v", updated)
	}

	if err := db.Tokens.AddRefresh(context.Background(), "self_delete_refresh", user.ID, database.NowMS()+3600*1000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	del := doJSON(t, h, "DELETE", "/me", nil, cookie)
	if del.Code != 200 {
		t.Fatalf("self delete status=%d body=%s", del.Code, del.Body.String())
	}
	if row, _ := db.Users.GetByID(context.Background(), user.ID); row != nil {
		t.Fatal("self delete should remove user")
	}
	if row, _ := db.Tokens.GetRefresh(context.Background(), "self_delete_refresh"); row != nil {
		t.Fatal("self delete should revoke refresh tokens")
	}

	admin := testutil.CreateUser(t, db, "selfadmin@test.com", "Password123", "SelfAdmin", true)
	adminAccess, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminDel := doJSON(t, h, "DELETE", "/me", nil, &http.Cookie{Name: "access_token", Value: adminAccess})
	if adminDel.Code != 403 {
		t.Fatalf("admin self delete should be 403, got %d", adminDel.Code)
	}
}

func TestRegistrationRestrictionsAndInviteConsumption(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	first := doJSON(t, h, "POST", "/register", map[string]any{"email": "admin-first@test.com", "password": "Password123", "username": "FirstAdmin"})
	if first.Code != 200 {
		t.Fatalf("first register status=%d body=%s", first.Code, first.Body.String())
	}
	firstUser, err := db.Users.GetByEmail(ctx, "admin-first@test.com")
	if err != nil || firstUser == nil || !firstUser.IsAdmin {
		t.Fatalf("first registered user should be admin: user=%#v err=%v", firstUser, err)
	}
	secondRegister := doJSON(t, h, "POST", "/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "SecondNormal"})
	if secondRegister.Code != 200 {
		t.Fatalf("second register status=%d body=%s", secondRegister.Code, secondRegister.Body.String())
	}
	secondUser, err := db.Users.GetByEmail(ctx, "second-normal@test.com")
	if err != nil || secondUser == nil || secondUser.IsAdmin {
		t.Fatalf("second registered user should not be admin: user=%#v err=%v", secondUser, err)
	}
	duplicateEmail := doJSON(t, h, "POST", "/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "DuplicateEmailUser"})
	if duplicateEmail.Code != 400 || !strings.Contains(duplicateEmail.Body.String(), "Email already registered") {
		t.Fatalf("duplicate email should be rejected, got %d body=%s", duplicateEmail.Code, duplicateEmail.Body.String())
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", true); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	for _, weak := range []string{"12345", "simplepass"} {
		resp := doJSON(t, h, "POST", "/register", map[string]any{"email": "weak_" + weak + "@test.com", "password": weak, "username": "Weak" + weak})
		if resp.Code != 400 {
			t.Fatalf("weak password %q should be rejected, got %d body=%s", weak, resp.Code, resp.Body.String())
		}
	}
	strong := doJSON(t, h, "POST", "/register", map[string]any{"email": "strong@test.com", "password": "StrongP@ss1", "username": "StrongUser"})
	if strong.Code != 200 {
		t.Fatalf("strong password should register, got %d body=%s", strong.Code, strong.Body.String())
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	for _, badEmail := range []string{"a@b", "a@x.com\r\nBcc: x@y.com", "notanemail"} {
		bad := doJSON(t, h, "POST", "/register", map[string]any{"email": badEmail, "password": "Password123!", "username": "SomeUser"})
		if bad.Code != 400 || !strings.Contains(bad.Body.String(), "Invalid email format") {
			t.Fatalf("invalid email %q should be rejected, got %d %s", badEmail, bad.Code, bad.Body.String())
		}
		if row, err := db.Users.GetByEmail(ctx, badEmail); err != nil || row != nil {
			t.Fatalf("invalid email registration should not create user: row=%#v err=%v", row, err)
		}
	}
	if err := db.Settings.Set(ctx, "allow_register", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	disabled := doJSON(t, h, "POST", "/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if disabled.Code != 403 {
		t.Fatalf("disabled register should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.Settings.Set(ctx, "allow_register", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", true); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	missingInvite := doJSON(t, h, "POST", "/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if missingInvite.Code != 400 {
		t.Fatalf("missing invite should be 400, got %d", missingInvite.Code)
	}
	if err := db.Invites.Create(ctx, "VALID_CODE", 1, "once"); err != nil {
		t.Fatal(err)
	}
	ok := doJSON(t, h, "POST", "/register", map[string]any{"email": "first@test.com", "password": "Password123", "username": "FirstUser", "invite": "VALID_CODE"})
	if ok.Code != 200 {
		t.Fatalf("valid invite register status=%d body=%s", ok.Code, ok.Body.String())
	}
	overuse := doJSON(t, h, "POST", "/register", map[string]any{"email": "second@test.com", "password": "Password123", "username": "SecondUser", "invite": "VALID_CODE"})
	if overuse.Code != 400 {
		t.Fatalf("overused invite should be 400, got %d body=%s", overuse.Code, overuse.Body.String())
	}
	second, _ := db.Users.GetByEmail(ctx, "second@test.com")
	if second != nil {
		t.Fatal("overused invite should not create user")
	}
}

func TestVerificationCodeRegisterAndResetPasswordHTTP(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	disabled := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if disabled.Code != 400 {
		t.Fatalf("verification disabled should be 400, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", 300); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)

	send := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if send.Code != 200 {
		t.Fatalf("send verification status=%d body=%s", send.Code, send.Body.String())
	}
	sendBody := parseJSON(t, send)
	if sendBody["ok"] != true || sendBody["ttl"] != float64(300) {
		t.Fatalf("unexpected verification response: %#v", sendBody)
	}
	code, err := redis.GetVerificationCode(ctx, "verify@test.com", "register")
	if err != nil {
		t.Fatalf("verification code missing err=%v", err)
	}
	if len(code) != 8 {
		t.Fatalf("bad verification code code=%q", code)
	}
	if _, _, ok, err := db.Verifications.GetCode(ctx, "verify@test.com", "register"); err != nil || ok {
		t.Fatalf("verification code should not be persisted in db: ok=%v err=%v", ok, err)
	}
	for _, ch := range code {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
			t.Fatalf("verification code contains invalid character: %q", code)
		}
	}

	badRegister := doJSON(t, h, "POST", "/register", map[string]any{"email": "verify@test.com", "password": "Password123!", "username": "VerifyUser", "code": "WRONG"})
	if badRegister.Code != 400 {
		t.Fatalf("wrong verification code should be 400, got %d body=%s", badRegister.Code, badRegister.Body.String())
	}
	register := doJSON(t, h, "POST", "/register", map[string]any{"email": "verify@test.com", "password": "Password123!", "username": "VerifyUser", "code": strings.ToLower(code)})
	if register.Code != 200 {
		t.Fatalf("verified register status=%d body=%s", register.Code, register.Body.String())
	}
	if _, err := redis.GetVerificationCode(ctx, "verify@test.com", "register"); err == nil {
		t.Fatal("register verification code should be deleted after use")
	}

	user := testutil.CreateUser(t, db, "reset@test.com", "OldPassword123!", "ResetUser", false)
	login := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "OldPassword123!"})
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}
	sendResetMissing := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "missing-reset@test.com", "type": "reset"})
	if sendResetMissing.Code != 200 || parseJSON(t, sendResetMissing)["ttl"] != float64(0) {
		t.Fatalf("missing reset target should return ok ttl=0, got %d %s", sendResetMissing.Code, sendResetMissing.Body.String())
	}
	sendReset := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": user.Email, "type": "reset"})
	if sendReset.Code != 200 {
		t.Fatalf("send reset status=%d body=%s", sendReset.Code, sendReset.Body.String())
	}
	resetCode, err := redis.GetVerificationCode(ctx, user.Email, "reset")
	if err != nil {
		t.Fatalf("reset code missing err=%v", err)
	}
	reset := doJSON(t, h, "POST", "/reset-password", map[string]any{"email": user.Email, "password": "NewPassword456!", "code": resetCode})
	if reset.Code != 200 {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	updated, _ := db.Users.GetByID(ctx, user.ID)
	if !util.VerifyPassword("NewPassword456!", updated.Password) {
		t.Fatal("reset password did not update password")
	}
	reuseRefresh := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
	if reuseRefresh.Code != 401 {
		t.Fatalf("old refresh should be revoked after reset, got %d", reuseRefresh.Code)
	}
	if _, err := redis.GetVerificationCode(ctx, user.Email, "reset"); err == nil {
		t.Fatal("reset verification code should be deleted after use")
	}
}
