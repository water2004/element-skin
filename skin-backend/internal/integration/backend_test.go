package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &b)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func parseJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode json %q: %v", rr.Body.String(), err)
	}
	return out
}

func cookieNamed(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func pngTexture(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func doMultipart(t *testing.T, h http.Handler, method, path string, fields map[string]string, fileField, fileName string, fileBytes []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		part, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestDatabaseInitScripts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	for _, table := range []string{"users", "settings", "fallback_endpoints"} {
		var got string
		if err := db.Pool.QueryRow(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=$1", table).Scan(&got); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
	v, err := db.GetSetting(ctx, "enable_skin_library", "")
	if err != nil {
		t.Fatal(err)
	}
	if v != "true" {
		t.Fatalf("enable_skin_library=%q", v)
	}
}

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
	if _, ok := meBody["profile_count"]; !ok {
		t.Fatalf("/me should include profile_count: %#v", meBody)
	}
	if _, ok := meBody["texture_count"]; !ok {
		t.Fatalf("/me should include texture_count: %#v", meBody)
	}
	if err := db.BanUser(context.Background(), user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	bannedMe := doJSON(t, h, "GET", "/me", nil, access)
	if bannedMe.Code != 200 {
		t.Fatalf("banned user should still access site API, got %d body=%s", bannedMe.Code, bannedMe.Body.String())
	}
	if err := db.UnbanUser(context.Background(), user.ID); err != nil {
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
	if ok, err := db.DeleteUser(context.Background(), deletedUser.ID); err != nil || !ok {
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
	if err := db.SetSetting(context.Background(), "site_name", "Public Name"); err != nil {
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

func TestYggdrasilAuthenticateJoinAndProfile(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "ygg@test.com", "YggPassword123", "YggUser", false)
	skin := "my_skin_hash"
	cape := "my_cape_hash"
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_id", "YggPlayer")
	if err := db.UpdateProfileSkin(context.Background(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileCape(context.Background(), profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileModel(context.Background(), profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}

	auth := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{
		"username": user.Email, "password": "YggPassword123", "requestUser": true,
	})
	if auth.Code != 200 {
		t.Fatalf("auth status=%d body=%s", auth.Code, auth.Body.String())
	}
	authBody := parseJSON(t, auth)
	if authBody["selectedProfile"].(map[string]any)["id"] != profile.ID {
		t.Fatalf("unexpected auth body: %#v", authBody)
	}
	userBody := authBody["user"].(map[string]any)
	if userBody["id"] != user.ID {
		t.Fatalf("requestUser should include user id: %#v", authBody)
	}
	propsUser := userBody["properties"].([]any)
	if len(propsUser) != 1 || propsUser[0].(map[string]any)["name"] != "preferredLanguage" || propsUser[0].(map[string]any)["value"] != "zh_CN" {
		t.Fatalf("requestUser should include preferredLanguage property: %#v", propsUser)
	}
	accessToken := authBody["accessToken"].(string)

	refresh := doJSON(t, h, "POST", "/authserver/refresh", map[string]any{
		"accessToken": accessToken, "clientToken": authBody["clientToken"], "requestUser": true,
	})
	if refresh.Code != 200 {
		t.Fatalf("ygg refresh status=%d body=%s", refresh.Code, refresh.Body.String())
	}
	refreshBody := parseJSON(t, refresh)
	newAccessToken := refreshBody["accessToken"].(string)
	if newAccessToken == "" || newAccessToken == accessToken {
		t.Fatalf("refresh should rotate access token: %#v", refreshBody)
	}
	if oldToken, _ := db.GetToken(context.Background(), accessToken); oldToken != nil {
		t.Fatal("old ygg access token should be deleted after refresh")
	}
	if newToken, _ := db.GetToken(context.Background(), newAccessToken); newToken == nil {
		t.Fatal("new ygg access token should be persisted after refresh")
	}
	if refreshBody["selectedProfile"].(map[string]any)["id"] != profile.ID || refreshBody["user"].(map[string]any)["id"] != user.ID {
		t.Fatalf("refresh should include selected profile and requestUser data: %#v", refreshBody)
	}
	accessToken = newAccessToken

	join := doJSON(t, h, "POST", "/sessionserver/session/minecraft/join", map[string]any{
		"accessToken": accessToken, "selectedProfile": profile.ID, "serverId": "server_1",
	})
	if join.Code != 204 {
		t.Fatalf("join status=%d body=%s", join.Code, join.Body.String())
	}
	hasJoined := doJSON(t, h, "GET", "/sessionserver/session/minecraft/hasJoined?username=YggPlayer&serverId=server_1", nil)
	if hasJoined.Code != 200 {
		t.Fatalf("hasJoined status=%d body=%s", hasJoined.Code, hasJoined.Body.String())
	}
	if parseJSON(t, hasJoined)["id"] != profile.ID {
		t.Fatal("hasJoined returned wrong profile")
	}

	profileResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+profile.ID, nil)
	if profileResp.Code != 200 {
		t.Fatalf("profile status=%d body=%s", profileResp.Code, profileResp.Body.String())
	}
	pbody := parseJSON(t, profileResp)
	props := pbody["properties"].([]any)
	texturesValue := props[0].(map[string]any)["value"].(string)
	decoded, err := base64.StdEncoding.DecodeString(texturesValue)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(decoded, []byte("my_skin_hash.png")) {
		t.Fatalf("textures payload missing skin hash: %s", decoded)
	}
	var texturesPayload map[string]any
	if err := json.Unmarshal(decoded, &texturesPayload); err != nil {
		t.Fatal(err)
	}
	textures := texturesPayload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	if skinPayload["metadata"].(map[string]any)["model"] != "slim" {
		t.Fatalf("slim skin should include model metadata: %#v", skinPayload)
	}
	if !strings.Contains(textures["CAPE"].(map[string]any)["url"].(string), "my_cape_hash.png") {
		t.Fatalf("cape url missing from textures payload: %#v", textures)
	}
	hasUploadable := false
	for _, raw := range props {
		if raw.(map[string]any)["name"] == "uploadableTextures" {
			hasUploadable = true
		}
	}
	if !hasUploadable {
		t.Fatalf("profile properties should include uploadableTextures: %#v", props)
	}

	defaultProfile := testutil.CreateProfile(t, db, user.ID, "default_profile_id", "DefaultModel")
	defaultSkin := "default_skin_hash"
	if err := db.UpdateProfileSkin(context.Background(), defaultProfile.ID, &defaultSkin); err != nil {
		t.Fatal(err)
	}
	defaultResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+defaultProfile.ID, nil)
	if defaultResp.Code != 200 {
		t.Fatalf("default profile status=%d body=%s", defaultResp.Code, defaultResp.Body.String())
	}
	defaultProps := parseJSON(t, defaultResp)["properties"].([]any)
	defaultDecoded, err := base64.StdEncoding.DecodeString(defaultProps[0].(map[string]any)["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var defaultPayload map[string]any
	if err := json.Unmarshal(defaultDecoded, &defaultPayload); err != nil {
		t.Fatal(err)
	}
	defaultTextures := defaultPayload["textures"].(map[string]any)
	if _, ok := defaultTextures["SKIN"].(map[string]any)["metadata"]; ok {
		t.Fatalf("default skin should not include metadata: %#v", defaultTextures["SKIN"])
	}
	if _, ok := defaultTextures["CAPE"]; ok {
		t.Fatalf("profile without cape should not include CAPE: %#v", defaultTextures)
	}
	signedResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+profile.ID+"?unsigned=false", nil)
	if signedResp.Code != 200 {
		t.Fatalf("signed profile status=%d body=%s", signedResp.Code, signedResp.Body.String())
	}
	signedProps := parseJSON(t, signedResp)["properties"].([]any)
	if signedProps[0].(map[string]any)["signature"] == "" {
		t.Fatal("signed profile should include a non-empty signature")
	}
	meta := doJSON(t, h, "GET", "/", nil)
	metaBody := parseJSON(t, meta)
	if metaBody["signaturePublickey"] == "" {
		t.Fatal("metadata should include a non-empty signature public key")
	}
	if metaBody["meta"].(map[string]any)["serverName"] != "皮肤站" {
		t.Fatalf("metadata should include site serverName: %#v", metaBody)
	}
	if len(metaBody["skinDomains"].([]any)) == 0 {
		t.Fatalf("metadata should include skinDomains: %#v", metaBody)
	}
}

func TestAdminAccessUsesDatabaseState(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "admin@test.com", "Password123", "Admin", true)
	normal := testutil.CreateUser(t, db, "normal@test.com", "Password123", "Normal", false)
	token, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminCookie := &http.Cookie{Name: "access_token", Value: token}

	users := doJSON(t, h, "GET", "/admin/users", nil, adminCookie)
	if users.Code != 200 {
		t.Fatalf("admin users status=%d body=%s", users.Code, users.Body.String())
	}
	if _, err := db.ToggleAdmin(context.Background(), admin.ID); err != nil {
		t.Fatal(err)
	}
	demoted := doJSON(t, h, "GET", "/admin/users", nil, adminCookie)
	if demoted.Code != 403 {
		t.Fatalf("demoted admin should be forbidden, got %d", demoted.Code)
	}

	normalToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, normal.ID, false, time.Hour)
	forbidden := doJSON(t, h, "GET", "/admin/users", nil, &http.Cookie{Name: "access_token", Value: normalToken})
	if forbidden.Code != 403 {
		t.Fatalf("normal user should be forbidden, got %d", forbidden.Code)
	}
}

func TestPublicSkinLibrarySearchAndWardrobeName(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	alice := testutil.CreateUser(t, db, "alice@test.com", "Password123", "ApiSearchAlice", false)
	bob := testutil.CreateUser(t, db, "bob@test.com", "Password123", "ApiSearchBob", false)
	charlie := testutil.CreateUser(t, db, "charlie@test.com", "Password123", "ApiSearchCharlie", false)
	if err := db.AddTextureToLibrary(context.Background(), alice.ID, "aaaa", "skin", "MagicSword", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), bob.ID, "bbbb", "skin", "DragonShield", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), charlie.ID, "cccc", "skin", "HolyArmor", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), charlie.ID, "dddd", "cape", "SharedName", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), alice.ID, "eeee", "skin", "UniquePrivateTex", false, "default"); err != nil {
		t.Fatal(err)
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
	if err := db.SetSetting(context.Background(), "enable_skin_library", false); err != nil {
		t.Fatal(err)
	}
	if disabled := doJSON(t, h, "GET", "/public/skin-library", nil); disabled.Code != 403 {
		t.Fatalf("disabled public library should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
}

func TestYggdrasilTokenStructStillUsable(t *testing.T) {
	_ = model.Token{AccessToken: "a", ClientToken: "c", UserID: "u", CreatedAt: time.Now().UnixMilli()}
}

func TestConcurrentRefreshSingleWinner(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "race@test.com", "Password123", "RaceUser", false)

	login := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "Password123"})
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}

	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
			codes <- rr.Code
		}()
	}
	wg.Wait()
	close(codes)

	seen := map[int]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[200] != 1 || seen[401] != 1 {
		t.Fatalf("expected one 200 and one 401, got %#v", seen)
	}
	row, err := db.GetRefreshToken(context.Background(), util.HashRefreshToken(refresh.Value))
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatal("old refresh token hash should be deleted")
	}
}

func TestDatabaseAtomicUserProfileInviteAndRefreshPrimitives(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "primitive@test.com", "Password123", "PrimitiveUser", false)
	now := database.NowMS()
	future := now + 7*24*3600*1000

	if err := db.AddRefreshToken(ctx, "hash_consume", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	row, err := db.ConsumeRefreshToken(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["user_id"] != user.ID || row["expires_at"] != future {
		t.Fatalf("unexpected consumed refresh row: %#v", row)
	}
	row, err = db.ConsumeRefreshToken(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatalf("refresh token should be one-shot, got %#v", row)
	}

	if err := db.AddRefreshToken(ctx, "hash_race", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	results := make(chan map[string]any, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := db.ConsumeRefreshToken(context.Background(), "hash_race")
			if err != nil {
				t.Errorf("consume refresh: %v", err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for got := range results {
		if got != nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected one refresh consume winner, got %d", winners)
	}

	atomicUser := model.User{ID: "atomic_user", Email: "atomic@test.com", Password: "hash", DisplayName: "AtomicUser"}
	atomicProfile := model.Profile{ID: "atomic_profile", UserID: atomicUser.ID, Name: "AtomicProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, atomicUser, atomicProfile, "", ""); err != nil {
		t.Fatal(err)
	}
	if u, _ := db.GetUserByID(ctx, atomicUser.ID); u == nil {
		t.Fatal("atomic user should be created")
	}
	if p, _ := db.GetProfileByID(ctx, atomicProfile.ID); p == nil {
		t.Fatal("atomic profile should be created")
	}

	conflictUser := model.User{ID: "orphan_user", Email: "orphan@test.com", Password: "hash", DisplayName: "OrphanUser"}
	conflictProfile := model.Profile{ID: "orphan_profile", UserID: conflictUser.ID, Name: "AtomicProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, conflictUser, conflictProfile, "", ""); err == nil {
		t.Fatal("profile name conflict should fail")
	}
	if u, _ := db.GetUserByID(ctx, conflictUser.ID); u != nil {
		t.Fatalf("profile conflict should roll back user insert: %#v", u)
	}
	if u, _ := db.GetUserByEmail(ctx, conflictUser.Email); u != nil {
		t.Fatalf("profile conflict should not leave user by email: %#v", u)
	}

	if err := db.CreateInvite(ctx, "GOOD_INVITE", 2, "good"); err != nil {
		t.Fatal(err)
	}
	invitedUser := model.User{ID: "invited_user", Email: "invited@test.com", Password: "hash", DisplayName: "InvitedUser"}
	invitedProfile := model.Profile{ID: "invited_profile", UserID: invitedUser.ID, Name: "InvitedProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, invitedUser, invitedProfile, "GOOD_INVITE", invitedUser.Email); err != nil {
		t.Fatal(err)
	}
	goodInvite, err := db.GetInvite(ctx, "GOOD_INVITE")
	if err != nil {
		t.Fatal(err)
	}
	if goodInvite == nil || goodInvite.UsedCount != 1 || goodInvite.UsedBy == nil || *goodInvite.UsedBy != invitedUser.Email {
		t.Fatalf("invite should be consumed with used_by: %#v", goodInvite)
	}

	if err := db.CreateInvite(ctx, "FULL_INVITE", 1, "full"); err != nil {
		t.Fatal(err)
	}
	firstUser := model.User{ID: "first_invite_user", Email: "first@test.com", Password: "hash", DisplayName: "FirstInviteUser"}
	firstProfile := model.Profile{ID: "first_invite_profile", UserID: firstUser.ID, Name: "FirstInviteProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, firstUser, firstProfile, "FULL_INVITE", firstUser.Email); err != nil {
		t.Fatal(err)
	}
	fullUser := model.User{ID: "full_invite_user", Email: "full@test.com", Password: "hash", DisplayName: "FullInviteUser"}
	fullProfile := model.Profile{ID: "full_invite_profile", UserID: fullUser.ID, Name: "FullInviteProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, fullUser, fullProfile, "FULL_INVITE", fullUser.Email); err != database.ErrInviteExhausted {
		t.Fatalf("expected ErrInviteExhausted, got %v", err)
	}
	if u, _ := db.GetUserByID(ctx, fullUser.ID); u != nil {
		t.Fatalf("exhausted invite should roll back user: %#v", u)
	}
	if p, _ := db.GetProfileByID(ctx, fullProfile.ID); p != nil {
		t.Fatalf("exhausted invite should roll back profile: %#v", p)
	}

	if err := db.CreateInvite(ctx, "RACE_INVITE", 1, "race"); err != nil {
		t.Fatal(err)
	}
	wins := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := model.User{ID: "race_user_" + strconv.Itoa(i), Email: "race" + strconv.Itoa(i) + "@test.com", Password: "hash", DisplayName: "RaceUser" + strconv.Itoa(i)}
			p := model.Profile{ID: "race_profile_" + strconv.Itoa(i), UserID: u.ID, Name: "RaceProfile" + strconv.Itoa(i), TextureModel: "default"}
			err := db.CreateUserWithProfile(context.Background(), u, p, "RACE_INVITE", u.Email)
			if err == nil {
				wins <- true
				return
			}
			if err != database.ErrInviteExhausted {
				t.Errorf("unexpected invite race error: %v", err)
			}
			wins <- false
		}()
	}
	wg.Wait()
	close(wins)
	successes := 0
	for ok := range wins {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one invite race winner, got %d", successes)
	}
	raceInvite, err := db.GetInvite(ctx, "RACE_INVITE")
	if err != nil {
		t.Fatal(err)
	}
	if raceInvite == nil || raceInvite.UsedCount != 1 {
		t.Fatalf("race invite should be consumed once: %#v", raceInvite)
	}
}

func TestDatabaseCursorPaginationCoverage(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	userIDs := map[string]bool{}
	for i := 0; i < 8; i++ {
		id := "page_user_" + strconv.Itoa(i)
		userIDs[id] = true
		u := model.User{ID: id, Email: "page" + strconv.Itoa(i) + "@test.com", Password: "hash", DisplayName: "Page User " + strconv.Itoa(i), PreferredLanguage: "en_US"}
		if err := db.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	seenUsers := map[string]bool{}
	lastID := ""
	for i := 0; i < 20; i++ {
		page, err := db.ListUsers(ctx, 3, lastID, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			id := raw["id"].(string)
			if seenUsers[id] {
				t.Fatalf("duplicate user page item %q", id)
			}
			seenUsers[id] = true
			if raw["email"] == "" || raw["display_name"] == "" {
				t.Fatalf("user pagination should map fields independently: %#v", raw)
			}
		}
		if page["has_next"] != true {
			break
		}
		lastID = page["next_key"].(map[string]any)["last_id"].(string)
	}
	for id := range userIDs {
		if !seenUsers[id] {
			t.Fatalf("user pagination missed %s, saw %#v", id, seenUsers)
		}
	}

	profileUser := testutil.CreateUser(t, db, "profiles-page@test.com", "Password123", "ProfilesPageUser", false)
	profileIDs := map[string]bool{}
	for i := 0; i < 5; i++ {
		id := "page_profile_" + strconv.Itoa(i)
		profileIDs[id] = true
		if err := db.CreateProfile(ctx, model.Profile{ID: id, UserID: profileUser.ID, Name: "PageProfile" + strconv.Itoa(i), TextureModel: "default"}); err != nil {
			t.Fatal(err)
		}
	}
	seenProfiles := map[string]bool{}
	lastID = ""
	for i := 0; i < 20; i++ {
		page, err := db.ListProfilesByUser(ctx, profileUser.ID, 2, lastID)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			id := raw["id"].(string)
			if seenProfiles[id] {
				t.Fatalf("duplicate profile page item %q", id)
			}
			seenProfiles[id] = true
		}
		if page["has_next"] != true {
			break
		}
		lastID = page["next_key"].(map[string]any)["last_id"].(string)
	}
	for id := range profileIDs {
		if !seenProfiles[id] {
			t.Fatalf("profile pagination missed %s, saw %#v", id, seenProfiles)
		}
	}

	baseTime := database.NowMS()
	inviteCodes := map[string]bool{}
	for i := 0; i < 6; i++ {
		code := "PAGE_INVITE_" + strconv.Itoa(i)
		inviteCodes[code] = true
		if err := db.CreateInvite(ctx, code, 1, "page"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `UPDATE invites SET created_at=$1 WHERE code=$2`, baseTime-int64(i*1000), code); err != nil {
			t.Fatal(err)
		}
	}
	seenInvites := map[string]bool{}
	var lastCreated *int64
	lastCode := ""
	for i := 0; i < 20; i++ {
		page, err := db.ListInvites(ctx, 2, lastCreated, lastCode)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			code := raw["code"].(string)
			if seenInvites[code] {
				t.Fatalf("duplicate invite page item %q", code)
			}
			seenInvites[code] = true
		}
		if page["has_next"] != true {
			break
		}
		next := page["next_key"].(map[string]any)
		v := next["last_created_at"].(int64)
		lastCreated = &v
		lastCode = next["last_code"].(string)
	}
	for code := range inviteCodes {
		if !seenInvites[code] {
			t.Fatalf("invite pagination missed %s, saw %#v", code, seenInvites)
		}
	}

	textureUser := testutil.CreateUser(t, db, "textures-page@test.com", "Password123", "TexturesPageUser", false)
	textureHashes := map[string]bool{}
	for i := 0; i < 5; i++ {
		hash := "page_skin_" + strconv.Itoa(i)
		textureHashes[hash] = true
		if err := db.AddTextureToLibrary(ctx, textureUser.ID, hash, "skin", "Page Skin "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.AddTextureToLibrary(ctx, textureUser.ID, "page_cape_"+strconv.Itoa(i), "cape", "Page Cape "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	seenTextures := map[string]bool{}
	var lastTextureCreated *int64
	lastHash := ""
	for i := 0; i < 20; i++ {
		page, err := db.ListUserTextures(ctx, textureUser.ID, "skin", 2, lastTextureCreated, lastHash)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			if raw["type"] != "skin" {
				t.Fatalf("type-filtered texture page returned non-skin: %#v", raw)
			}
			hash := raw["hash"].(string)
			if seenTextures[hash] {
				t.Fatalf("duplicate texture page item %q", hash)
			}
			seenTextures[hash] = true
		}
		if page["has_next"] != true {
			break
		}
		next := page["next_key"].(map[string]any)
		v := next["last_created_at"].(int64)
		lastTextureCreated = &v
		lastHash = next["last_hash"].(string)
	}
	for hash := range textureHashes {
		if !seenTextures[hash] {
			t.Fatalf("texture pagination missed %s, saw %#v", hash, seenTextures)
		}
	}
}

func TestDatabaseUserProfileTokenAndTextureCRUD(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "crud@test.com", "Password123", "CrudUser", false)

	if count, err := db.CountUsers(ctx); err != nil || count != 1 {
		t.Fatalf("CountUsers=%d err=%v", count, err)
	}
	if taken, err := db.IsDisplayNameTaken(ctx, "CrudUser", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: %v", err)
	}
	if err := db.UpdateUser(ctx, user.ID, map[string]any{"email": "new@crud.com", "display_name": "NewCrud", "preferred_language": "en_US"}); err != nil {
		t.Fatal(err)
	}
	updated, _ := db.GetUserByID(ctx, user.ID)
	if updated.Email != "new@crud.com" || updated.DisplayName != "NewCrud" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}
	if err := db.BanUser(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.IsBanned(ctx, user.ID); err != nil || !banned {
		t.Fatalf("expected banned user: %v", err)
	}
	if err := db.UnbanUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if banned, _ := db.IsBanned(ctx, user.ID); banned {
		t.Fatal("expected unbanned user")
	}

	profile := testutil.CreateProfile(t, db, user.ID, "crud_profile", "CrudPlayer")
	skin := "skin_hash"
	cape := "cape_hash"
	if err := db.UpdateProfileSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileCape(ctx, profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	gotProfile, _ := db.GetProfileByID(ctx, profile.ID)
	if *gotProfile.SkinHash != skin || *gotProfile.CapeHash != cape || gotProfile.TextureModel != "slim" {
		t.Fatalf("unexpected profile: %#v", gotProfile)
	}

	token := model.Token{AccessToken: "acc_token", ClientToken: "cli", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := db.AddToken(ctx, token); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.GetToken(ctx, "acc_token"); got == nil {
		t.Fatal("token missing")
	}
	if ok, err := db.DeleteProfileCascade(ctx, profile.ID); err != nil || !ok {
		t.Fatalf("DeleteProfileCascade ok=%v err=%v", ok, err)
	}
	if got, _ := db.GetToken(ctx, "acc_token"); got != nil {
		t.Fatal("profile token should be cascaded")
	}

	if err := db.AddTextureToLibrary(ctx, user.ID, "texhash", "skin", "MySkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if info, _ := db.GetTextureInfo(ctx, user.ID, "texhash", "skin"); info["note"] != "MySkin" || info["is_public"].(int) != 1 {
		t.Fatalf("unexpected texture info: %#v", info)
	}
	if err := db.UpdateTextureNote(ctx, user.ID, "texhash", "skin", "NewNote"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTexturePublic(ctx, user.ID, "texhash", "skin", false); err != nil {
		t.Fatal(err)
	}
	other := testutil.CreateUser(t, db, "other@test.com", "Password123", "Other", false)
	ok, err := db.AddTextureToWardrobe(ctx, other.ID, "texhash")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("other user should not add private texture")
	}
	if err := db.UpdateTexturePublic(ctx, user.ID, "texhash", "skin", true); err != nil {
		t.Fatal(err)
	}
	ok, err = db.AddTextureToWardrobe(ctx, other.ID, "texhash")
	if err != nil || !ok {
		t.Fatalf("public wardrobe add ok=%v err=%v", ok, err)
	}
	if info, _ := db.GetTextureInfo(ctx, other.ID, "texhash", "skin"); info == nil || info["is_public"].(int) != 2 {
		t.Fatalf("wardrobe copy should use is_public=2, got %#v", info)
	}

	modelHash := "modelhash"
	if err := db.AddTextureToLibrary(ctx, user.ID, modelHash, "skin", "ModelSkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	modelProfile := testutil.CreateProfile(t, db, user.ID, "model_profile", "ModelTester")
	if err := db.UpdateProfileSkin(ctx, modelProfile.ID, &modelHash); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTextureModel(ctx, user.ID, modelHash, "skin", "slim"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ := db.GetProfileByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("owner model update should cascade to profile, got %#v", updatedModelProfile)
	}
	otherModelUser := testutil.CreateUser(t, db, "other-model@test.com", "Password123", "OtherModel", false)
	if ok, err := db.AddTextureToWardrobe(ctx, otherModelUser.ID, modelHash); err != nil || !ok {
		t.Fatalf("other model wardrobe add ok=%v err=%v", ok, err)
	}
	if err := db.UpdateTextureModel(ctx, otherModelUser.ID, modelHash, "skin", "default"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ = db.GetProfileByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("non-uploader model update should not cascade owner profile, got %#v", updatedModelProfile)
	}

	legacyHash := "legacyhash"
	if err := db.AddTextureToLibrary(ctx, user.ID, legacyHash, "skin", "LegacySkin", false, "default"); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.DeleteTextureFromLibrary(ctx, user.ID, legacyHash, "skin"); err != nil || !ok {
		t.Fatalf("delete legacy texture ok=%v err=%v", ok, err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO skin_library (skin_hash,texture_type,is_public,uploader,model,name,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, legacyHash, "skin", 0, user.ID, "default", "LegacySkin", int64(1234567890)); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.AddTextureToWardrobe(ctx, user.ID, legacyHash); err != nil || !ok {
		t.Fatalf("owner should recover legacy private texture ok=%v err=%v", ok, err)
	}
	if info, _ := db.GetTextureInfo(ctx, user.ID, legacyHash, "skin"); info == nil || info["is_public"].(int) != 1 {
		t.Fatalf("owner recovered texture should use is_public=1, got %#v", info)
	}
}

func TestYggdrasilProfileNameLoginAndLookupMiss(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "multi@test.com", "YggPassword123", "MultiUser", false)
	p1 := testutil.CreateProfile(t, db, user.ID, "profile_1", "PlayerOne")
	_ = testutil.CreateProfile(t, db, user.ID, "profile_2", "PlayerTwo")

	byName := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{"username": "PlayerOne", "password": "YggPassword123"})
	if byName.Code != 200 {
		t.Fatalf("profile-name auth status=%d body=%s", byName.Code, byName.Body.String())
	}
	body := parseJSON(t, byName)
	if body["selectedProfile"].(map[string]any)["id"] != p1.ID {
		t.Fatalf("wrong selected profile: %#v", body)
	}
	if len(body["availableProfiles"].([]any)) != 1 {
		t.Fatalf("profile-name login should return one profile: %#v", body)
	}

	byEmail := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{"username": user.Email, "password": "YggPassword123"})
	if byEmail.Code != 200 {
		t.Fatalf("email auth status=%d body=%s", byEmail.Code, byEmail.Body.String())
	}
	emailBody := parseJSON(t, byEmail)
	if _, ok := emailBody["selectedProfile"]; ok {
		t.Fatalf("email login with multiple profiles should not select one: %#v", emailBody)
	}
	if len(emailBody["availableProfiles"].([]any)) != 2 {
		t.Fatalf("email login should return two profiles: %#v", emailBody)
	}

	miss := doJSON(t, h, "GET", "/api/profiles/minecraft/NoSuchPlayer", nil)
	if miss.Code != 204 {
		t.Fatalf("lookup miss should be 204, got %d", miss.Code)
	}
}

func TestSiteProfileTextureHTTPFlows(t *testing.T) {
	db, h := testutil.NewTestApp(t)
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

	if err := db.SetSetting(context.Background(), "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	offline := doJSON(t, h, "POST", "/me/profiles", map[string]any{"name": "OfflinePlayerA", "model": "default"}, cookie)
	if offline.Code != 200 {
		t.Fatalf("offline profile status=%d body=%s", offline.Code, offline.Body.String())
	}
	if parseJSON(t, offline)["id"] != util.OfflineUUIDNoDash("OfflinePlayerA") {
		t.Fatalf("offline profile should use offline UUID: %s", offline.Body.String())
	}
	if err := db.SetSetting(context.Background(), "profile_uuid_mode", "random"); err != nil {
		t.Fatal(err)
	}

	create := doJSON(t, h, "POST", "/me/profiles", map[string]any{"name": "ApiPlayer", "model": "default"}, cookie)
	if create.Code != 200 {
		t.Fatalf("create profile status=%d body=%s", create.Code, create.Body.String())
	}
	profileID := parseJSON(t, create)["id"].(string)
	for i := 0; i < 5; i++ {
		if err := db.CreateProfile(context.Background(), model.Profile{ID: "http_profile_" + strconv.Itoa(i), UserID: user.ID, Name: "HTTPProfile_" + strconv.Itoa(i), TextureModel: "default"}); err != nil {
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
	p, _ := db.GetProfileByID(context.Background(), profileID)
	if p.Name != "NewFancyName" {
		t.Fatalf("profile not renamed: %#v", p)
	}

	if err := db.AddTextureToLibrary(context.Background(), user.ID, "apply_hash", "skin", "ApplySkin", false, "slim"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := db.AddTextureToLibrary(context.Background(), user.ID, "http_tex_"+strconv.Itoa(i), "skin", "HTTP Texture "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
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
			if _, ok := item["hash"]; !ok {
				t.Fatalf("/me/textures item should include hash: %#v", item)
			}
			hash := item["hash"].(string)
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
	for i := 0; i < 3; i++ {
		hash := "http_tex_" + strconv.Itoa(i)
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
	if err := db.AddTextureToLibrary(context.Background(), libraryOwner.ID, "lib_tex_hash_123", "skin", "Epic Skin Name", true, "default"); err != nil {
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
	addedInfo, _ := db.GetTextureInfo(context.Background(), user.ID, "lib_tex_hash_123", "skin")
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
	p, _ = db.GetProfileByID(context.Background(), profileID)
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
	info, _ := db.GetTextureInfo(context.Background(), user.ID, "apply_hash", "skin")
	if info["note"] != "RenamedSkin" || info["is_public"].(int) != 1 {
		t.Fatalf("texture update did not persist: %#v", info)
	}

	clear := doJSON(t, h, "DELETE", "/me/profiles/"+profileID+"/skin", nil, cookie)
	if clear.Code != 200 {
		t.Fatalf("clear status=%d body=%s", clear.Code, clear.Body.String())
	}
	p, _ = db.GetProfileByID(context.Background(), profileID)
	if p.SkinHash != nil {
		t.Fatalf("skin should be cleared: %#v", p)
	}

	del := doJSON(t, h, "DELETE", "/me/profiles/"+profileID, nil, cookie)
	if del.Code != 200 {
		t.Fatalf("delete profile status=%d body=%s", del.Code, del.Body.String())
	}
	p, _ = db.GetProfileByID(context.Background(), profileID)
	if p != nil {
		t.Fatal("profile should be deleted")
	}
}

func TestTextureUploadAndYggdrasilTextureRoutes(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "upload@test.com", "Password123", "Uploader", false)
	profile := testutil.CreateProfile(t, db, user.ID, "upload_profile", "UploadPlayer")
	access, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: access}

	upload := doMultipart(t, h, "POST", "/me/textures", map[string]string{
		"texture_type": "skin",
		"note":         "API Upload",
		"is_public":    "true",
		"model":        "default",
	}, "file", "skin.png", pngTexture(t, 64, 64), cookie)
	if upload.Code != 200 {
		t.Fatalf("site texture upload status=%d body=%s", upload.Code, upload.Body.String())
	}
	hash := parseJSON(t, upload)["hash"].(string)
	info, _ := db.GetTextureInfo(context.Background(), user.ID, hash, "skin")
	if info == nil || info["note"] != "API Upload" || info["is_public"].(int) != 1 {
		t.Fatalf("uploaded texture not persisted: %#v", info)
	}

	oversized := doMultipart(t, h, "POST", "/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "too-large.png", bytes.Repeat([]byte("x"), 16<<20+1), cookie)
	if oversized.Code != 400 || !strings.Contains(oversized.Body.String(), "File too large") {
		t.Fatalf("oversized texture upload should be rejected, got %d body=%s", oversized.Code, oversized.Body.String())
	}

	missingBearer := doMultipart(t, h, "PUT", "/api/user/profile/"+profile.ID+"/skin", nil, "file", "skin.png", pngTexture(t, 64, 64))
	if missingBearer.Code != 401 {
		t.Fatalf("missing bearer should be 401, got %d", missingBearer.Code)
	}

	token := model.Token{AccessToken: "texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := db.AddToken(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("model", "slim")
	part, _ := mw.CreateFormFile("file", "skin.png")
	_, _ = part.Write(pngTexture(t, 64, 64))
	_ = mw.Close()
	yggReq := httptest.NewRequest("PUT", "/api/user/profile/"+profile.ID+"/skin", &b)
	yggReq.Header.Set("Content-Type", mw.FormDataContentType())
	yggReq.Header.Set("Authorization", "Bearer texture_token")
	yggRR := httptest.NewRecorder()
	h.ServeHTTP(yggRR, yggReq)
	if yggRR.Code != 200 {
		t.Fatalf("ygg texture upload status=%d body=%s", yggRR.Code, yggRR.Body.String())
	}
	p, _ := db.GetProfileByID(context.Background(), profile.ID)
	if p.SkinHash == nil || p.TextureModel != "slim" {
		t.Fatalf("ygg upload did not apply skin/model: %#v", p)
	}

	delReq := httptest.NewRequest("DELETE", "/api/user/profile/"+profile.ID+"/skin", nil)
	delReq.Header.Set("Authorization", "Bearer texture_token")
	delRR := httptest.NewRecorder()
	h.ServeHTTP(delRR, delReq)
	if delRR.Code != 204 {
		t.Fatalf("ygg delete status=%d body=%s", delRR.Code, delRR.Body.String())
	}
	p, _ = db.GetProfileByID(context.Background(), profile.ID)
	if p.SkinHash != nil {
		t.Fatalf("skin should be cleared: %#v", p)
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
	updated, _ := db.GetProfileByID(context.Background(), profile.ID)
	if updated.SkinHash == nil || updated.TextureModel != "slim" {
		t.Fatalf("direct upload did not apply texture/model: %#v", updated)
	}

	if err := db.AddRefreshToken(context.Background(), "self_delete_refresh", user.ID, database.NowMS()+3600*1000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	del := doJSON(t, h, "DELETE", "/me", nil, cookie)
	if del.Code != 200 {
		t.Fatalf("self delete status=%d body=%s", del.Code, del.Body.String())
	}
	if row, _ := db.GetUserByID(context.Background(), user.ID); row != nil {
		t.Fatal("self delete should remove user")
	}
	if row, _ := db.GetRefreshToken(context.Background(), "self_delete_refresh"); row != nil {
		t.Fatal("self delete should revoke refresh tokens")
	}

	admin := testutil.CreateUser(t, db, "selfadmin@test.com", "Password123", "SelfAdmin", true)
	adminAccess, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminDel := doJSON(t, h, "DELETE", "/me", nil, &http.Cookie{Name: "access_token", Value: adminAccess})
	if adminDel.Code != 403 {
		t.Fatalf("admin self delete should be 403, got %d", adminDel.Code)
	}
}

func TestAdminProfilesTexturesInvitesHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "root@test.com", "Password123", "Root", true)
	user := testutil.CreateUser(t, db, "owned@test.com", "Password123", "Owned", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}

	profile := testutil.CreateProfile(t, db, user.ID, "admin_profile", "AdminList1")
	otherProfile := testutil.CreateProfile(t, db, user.ID, "admin_profile_other", "OtherName")
	forbiddenProfiles := doJSON(t, h, "GET", "/admin/profiles", nil, userCookie)
	if forbiddenProfiles.Code != 403 {
		t.Fatalf("non-admin profiles list should be 403, got %d", forbiddenProfiles.Code)
	}
	profiles := doJSON(t, h, "GET", "/admin/profiles?q=AdminList1", nil, adminCookie)
	if profiles.Code != 200 {
		t.Fatalf("admin profiles status=%d body=%s", profiles.Code, profiles.Body.String())
	}
	items := parseJSON(t, profiles)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "AdminList1" {
		t.Fatalf("unexpected admin profiles: %#v", items)
	}

	patchProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": "AdminRenamed"}, adminCookie)
	if patchProfile.Code != 200 {
		t.Fatalf("admin patch profile status=%d body=%s", patchProfile.Code, patchProfile.Body.String())
	}
	p, _ := db.GetProfileByID(context.Background(), profile.ID)
	if p.Name != "AdminRenamed" {
		t.Fatal("admin profile rename did not persist")
	}
	duplicateProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": otherProfile.Name}, adminCookie)
	if duplicateProfile.Code != 409 {
		t.Fatalf("admin duplicate profile name should be 409, got %d body=%s", duplicateProfile.Code, duplicateProfile.Body.String())
	}
	forbiddenPatchProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": "Nope"}, userCookie)
	if forbiddenPatchProfile.Code != 403 {
		t.Fatalf("non-admin profile patch should be 403, got %d", forbiddenPatchProfile.Code)
	}
	missingProfileDelete := doJSON(t, h, "DELETE", "/admin/profiles/missing-profile", nil, adminCookie)
	if missingProfileDelete.Code != 404 {
		t.Fatalf("missing admin profile delete should be 404, got %d", missingProfileDelete.Code)
	}
	skinHash := "skinhash"
	if err := db.UpdateProfileSkin(context.Background(), profile.ID, &skinHash); err != nil {
		t.Fatal(err)
	}
	capeHash := "capehash"
	if err := db.UpdateProfileCape(context.Background(), profile.ID, &capeHash); err != nil {
		t.Fatal(err)
	}
	forbiddenClearSkin := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/skin", map[string]any{"hash": nil}, userCookie)
	if forbiddenClearSkin.Code != 403 {
		t.Fatalf("non-admin clear skin should be 403, got %d", forbiddenClearSkin.Code)
	}
	clearSkin := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/skin", map[string]any{"hash": nil}, adminCookie)
	if clearSkin.Code != 200 {
		t.Fatalf("admin clear skin status=%d body=%s", clearSkin.Code, clearSkin.Body.String())
	}
	p, _ = db.GetProfileByID(context.Background(), profile.ID)
	if p.SkinHash != nil || p.CapeHash == nil || *p.CapeHash != capeHash {
		t.Fatalf("clearing skin should not affect cape: %#v", p)
	}
	clearCape := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/cape", map[string]any{"hash": nil}, adminCookie)
	if clearCape.Code != 200 {
		t.Fatalf("admin clear cape status=%d body=%s", clearCape.Code, clearCape.Body.String())
	}
	p, _ = db.GetProfileByID(context.Background(), profile.ID)
	if p.CapeHash != nil {
		t.Fatal("admin cape clear did not persist")
	}
	missingClearCape := doJSON(t, h, "PATCH", "/admin/profiles/missing-profile/cape", map[string]any{"hash": nil}, adminCookie)
	if missingClearCape.Code != 404 {
		t.Fatalf("missing clear cape should be 404, got %d", missingClearCape.Code)
	}

	if err := db.AddTextureToLibrary(context.Background(), user.ID, "adm_hash", "skin", "AdminTexture", true, "default"); err != nil {
		t.Fatal(err)
	}
	forbiddenTextures := doJSON(t, h, "GET", "/admin/textures", nil, userCookie)
	if forbiddenTextures.Code != 403 {
		t.Fatalf("non-admin textures list should be 403, got %d", forbiddenTextures.Code)
	}
	textures := doJSON(t, h, "GET", "/admin/textures?q=AdminTexture", nil, adminCookie)
	if textures.Code != 200 {
		t.Fatalf("admin textures status=%d body=%s", textures.Code, textures.Body.String())
	}
	textureItems := parseJSON(t, textures)["items"].([]any)
	if len(textureItems) != 1 || textureItems[0].(map[string]any)["hash"] != "adm_hash" {
		t.Fatalf("unexpected admin textures: %#v", textureItems)
	}
	patchTex := doJSON(t, h, "PATCH", "/admin/textures/adm_hash", map[string]any{"is_public": 0, "note": "AdminRenamedTexture"}, adminCookie)
	if patchTex.Code != 200 {
		t.Fatalf("admin patch texture status=%d body=%s", patchTex.Code, patchTex.Body.String())
	}
	info, _ := db.GetTextureInfo(context.Background(), user.ID, "adm_hash", "skin")
	if info["is_public"].(int) != 0 || info["note"] != "AdminRenamedTexture" {
		t.Fatalf("admin texture patch did not persist: %#v", info)
	}
	forbiddenPatchTexture := doJSON(t, h, "PATCH", "/admin/textures/adm_hash", map[string]any{"is_public": 1}, userCookie)
	if forbiddenPatchTexture.Code != 403 {
		t.Fatalf("non-admin texture patch should be 403, got %d", forbiddenPatchTexture.Code)
	}
	forbiddenDeleteTexture := doJSON(t, h, "DELETE", "/admin/textures/adm_hash?user_id="+user.ID+"&type=skin", nil, userCookie)
	if forbiddenDeleteTexture.Code != 403 {
		t.Fatalf("non-admin texture delete should be 403, got %d", forbiddenDeleteTexture.Code)
	}
	delTex := doJSON(t, h, "DELETE", "/admin/textures/adm_hash?user_id="+user.ID+"&type=skin", nil, adminCookie)
	if delTex.Code != 200 {
		t.Fatalf("admin delete texture status=%d body=%s", delTex.Code, delTex.Body.String())
	}
	info, _ = db.GetTextureInfo(context.Background(), user.ID, "adm_hash", "skin")
	if info != nil {
		t.Fatal("texture should be deleted from user library")
	}

	createInvite := doJSON(t, h, "POST", "/admin/invites", map[string]any{"code": "INV_HTTP", "total_uses": 5, "note": "API Code"}, adminCookie)
	if createInvite.Code != 200 {
		t.Fatalf("create invite status=%d body=%s", createInvite.Code, createInvite.Body.String())
	}
	invites := doJSON(t, h, "GET", "/admin/invites", nil, adminCookie)
	if invites.Code != 200 {
		t.Fatalf("list invites status=%d body=%s", invites.Code, invites.Body.String())
	}
	found := false
	for _, raw := range parseJSON(t, invites)["items"].([]any) {
		if raw.(map[string]any)["code"] == "INV_HTTP" {
			found = true
		}
	}
	if !found {
		t.Fatal("created invite not listed")
	}
	delInvite := doJSON(t, h, "DELETE", "/admin/invites/INV_HTTP", nil, adminCookie)
	if delInvite.Code != 200 {
		t.Fatalf("delete invite status=%d body=%s", delInvite.Code, delInvite.Body.String())
	}
	inv, _ := db.GetInvite(context.Background(), "INV_HTTP")
	if inv != nil {
		t.Fatal("invite should be deleted")
	}
}

func TestAdminUserControlsHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "admin-controls@test.com", "Password123", "AdminControls", true)
	user := testutil.CreateUser(t, db, "normal-controls@test.com", "Password123", "NormalControls", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}

	search := doJSON(t, h, "GET", "/admin/users?q=NormalControls", nil, adminCookie)
	if search.Code != 200 {
		t.Fatalf("admin user search status=%d body=%s", search.Code, search.Body.String())
	}
	items := parseJSON(t, search)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != user.ID {
		t.Fatalf("unexpected user search result: %#v", items)
	}

	invalidCursor := doJSON(t, h, "GET", "/admin/users?cursor=garbage!!", nil, adminCookie)
	if invalidCursor.Code != 400 {
		t.Fatalf("invalid admin user cursor should be 400, got %d", invalidCursor.Code)
	}

	selfToggle := doJSON(t, h, "POST", "/admin/users/"+admin.ID+"/toggle-admin", nil, adminCookie)
	if selfToggle.Code != 403 {
		t.Fatalf("self toggle should be 403, got %d body=%s", selfToggle.Code, selfToggle.Body.String())
	}
	toggled := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/toggle-admin", nil, adminCookie)
	if toggled.Code != 200 || parseJSON(t, toggled)["is_admin"] != true {
		t.Fatalf("toggle admin status=%d body=%s", toggled.Code, toggled.Body.String())
	}
	row, _ := db.GetUserByID(context.Background(), user.ID)
	if row == nil || !row.IsAdmin {
		t.Fatalf("user should now be admin: %#v", row)
	}
	toggledBack := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/toggle-admin", nil, adminCookie)
	if toggledBack.Code != 200 || parseJSON(t, toggledBack)["is_admin"] != false {
		t.Fatalf("toggle back status=%d body=%s", toggledBack.Code, toggledBack.Body.String())
	}

	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	ban := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/ban", map[string]any{"banned_until": bannedUntil}, adminCookie)
	if ban.Code != 200 {
		t.Fatalf("ban status=%d body=%s", ban.Code, ban.Body.String())
	}
	if banned, _ := db.IsBanned(context.Background(), user.ID); !banned {
		t.Fatal("user should be banned")
	}
	unban := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/unban", nil, adminCookie)
	if unban.Code != 200 {
		t.Fatalf("unban status=%d body=%s", unban.Code, unban.Body.String())
	}
	if banned, _ := db.IsBanned(context.Background(), user.ID); banned {
		t.Fatal("user should be unbanned")
	}

	now := database.NowMS()
	refreshHashes := []string{util.HashRefreshToken("admin-reset-1"), util.HashRefreshToken("admin-reset-2")}
	for _, hsh := range refreshHashes {
		if err := db.AddRefreshToken(context.Background(), hsh, user.ID, now+3600*1000, now); err != nil {
			t.Fatal(err)
		}
	}
	reset := doJSON(t, h, "POST", "/admin/users/reset-password", map[string]any{"user_id": user.ID, "new_password": "NewStr0ngPass!"}, adminCookie)
	if reset.Code != 200 {
		t.Fatalf("reset password status=%d body=%s", reset.Code, reset.Body.String())
	}
	updated, _ := db.GetUserByID(context.Background(), user.ID)
	if !util.VerifyPassword("NewStr0ngPass!", updated.Password) {
		t.Fatal("password was not updated")
	}
	for _, hsh := range refreshHashes {
		if row, _ := db.GetRefreshToken(context.Background(), hsh); row != nil {
			t.Fatal("admin reset should revoke refresh tokens")
		}
	}
	missingReset := doJSON(t, h, "POST", "/admin/users/reset-password", map[string]any{"user_id": "missing", "new_password": "x"}, adminCookie)
	if missingReset.Code != 404 {
		t.Fatalf("missing reset should be 404, got %d", missingReset.Code)
	}

	profile := testutil.CreateProfile(t, db, user.ID, "delete_user_profile", "DeleteUserProfile")
	token := model.Token{AccessToken: "delete-user-token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: now}
	if err := db.AddToken(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), user.ID, "delete_user_texture", "skin", "DeleteUserTex", true, "default"); err != nil {
		t.Fatal(err)
	}
	del := doJSON(t, h, "DELETE", "/admin/users/"+user.ID, nil, adminCookie)
	if del.Code != 200 {
		t.Fatalf("delete user status=%d body=%s", del.Code, del.Body.String())
	}
	if row, _ := db.GetUserByID(context.Background(), user.ID); row != nil {
		t.Fatal("user should be deleted")
	}
	if p, _ := db.GetProfileByID(context.Background(), profile.ID); p != nil {
		t.Fatal("user profiles should be deleted")
	}
	if tok, _ := db.GetToken(context.Background(), "delete-user-token"); tok != nil {
		t.Fatal("user tokens should be deleted")
	}
	if ok, _ := db.VerifyTextureOwnership(context.Background(), user.ID, "delete_user_texture", "skin"); ok {
		t.Fatal("user textures should be deleted")
	}
	missingDelete := doJSON(t, h, "DELETE", "/admin/users/"+user.ID, nil, adminCookie)
	if missingDelete.Code != 404 {
		t.Fatalf("missing delete should be 404, got %d", missingDelete.Code)
	}
}

func TestAdminTextureValidationEdges(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "edgeadmin@test.com", "Password123", "EdgeAdmin", true)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}

	invalidPublic := doJSON(t, h, "PATCH", "/admin/textures/somehash", map[string]any{"is_public": 2}, adminCookie)
	if invalidPublic.Code != 400 {
		t.Fatalf("invalid is_public should be 400, got %d body=%s", invalidPublic.Code, invalidPublic.Body.String())
	}
	missingPublic := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"is_public": 0}, adminCookie)
	if missingPublic.Code != 404 {
		t.Fatalf("missing texture public update should be 404, got %d body=%s", missingPublic.Code, missingPublic.Body.String())
	}
	missingNote := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"note": "missing"}, adminCookie)
	if missingNote.Code != 404 {
		t.Fatalf("missing texture note update should be 404, got %d body=%s", missingNote.Code, missingNote.Body.String())
	}
	invalidModel := doJSON(t, h, "PATCH", "/admin/textures/somehash", map[string]any{"model": "invalid"}, adminCookie)
	if invalidModel.Code != 400 {
		t.Fatalf("invalid model should be 400, got %d body=%s", invalidModel.Code, invalidModel.Body.String())
	}
	missingModel := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"model": "slim"}, adminCookie)
	if missingModel.Code != 404 {
		t.Fatalf("missing texture model update should be 404, got %d body=%s", missingModel.Code, missingModel.Body.String())
	}
	missingUser := doJSON(t, h, "DELETE", "/admin/textures/somehash?type=skin", nil, adminCookie)
	if missingUser.Code != 400 {
		t.Fatalf("per-user delete without user_id should be 400, got %d body=%s", missingUser.Code, missingUser.Body.String())
	}

	user1 := testutil.CreateUser(t, db, "tex1@test.com", "Password123", "TexOne", false)
	user2 := testutil.CreateUser(t, db, "tex2@test.com", "Password123", "TexTwo", false)
	if err := db.AddTextureToLibrary(context.Background(), user1.ID, "force_hash", "skin", "Force", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(context.Background(), user2.ID, "force_hash", "skin", "Copy", true, "default"); err != nil {
		t.Fatal(err)
	}
	force := doJSON(t, h, "DELETE", "/admin/textures/force_hash?type=skin&force=true", nil, adminCookie)
	if force.Code != 200 {
		t.Fatalf("force delete status=%d body=%s", force.Code, force.Body.String())
	}
	if ok, _ := db.VerifyTextureOwnership(context.Background(), user1.ID, "force_hash", "skin"); ok {
		t.Fatal("force delete should remove user1 reference")
	}
	if ok, _ := db.VerifyTextureOwnership(context.Background(), user2.ID, "force_hash", "skin"); ok {
		t.Fatal("force delete should remove user2 reference")
	}
}

func TestRegistrationRestrictionsAndInviteConsumption(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	ctx := context.Background()
	first := doJSON(t, h, "POST", "/register", map[string]any{"email": "admin-first@test.com", "password": "Password123", "username": "FirstAdmin"})
	if first.Code != 200 {
		t.Fatalf("first register status=%d body=%s", first.Code, first.Body.String())
	}
	firstUser, err := db.GetUserByEmail(ctx, "admin-first@test.com")
	if err != nil || firstUser == nil || !firstUser.IsAdmin {
		t.Fatalf("first registered user should be admin: user=%#v err=%v", firstUser, err)
	}
	secondRegister := doJSON(t, h, "POST", "/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "SecondNormal"})
	if secondRegister.Code != 200 {
		t.Fatalf("second register status=%d body=%s", secondRegister.Code, secondRegister.Body.String())
	}
	secondUser, err := db.GetUserByEmail(ctx, "second-normal@test.com")
	if err != nil || secondUser == nil || secondUser.IsAdmin {
		t.Fatalf("second registered user should not be admin: user=%#v err=%v", secondUser, err)
	}
	duplicateEmail := doJSON(t, h, "POST", "/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "DuplicateEmailUser"})
	if duplicateEmail.Code != 400 || !strings.Contains(duplicateEmail.Body.String(), "Email already registered") {
		t.Fatalf("duplicate email should be rejected, got %d body=%s", duplicateEmail.Code, duplicateEmail.Body.String())
	}
	if err := db.SetSetting(ctx, "enable_strong_password_check", true); err != nil {
		t.Fatal(err)
	}
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
	if err := db.SetSetting(ctx, "enable_strong_password_check", false); err != nil {
		t.Fatal(err)
	}
	for _, badEmail := range []string{"a@b", "a@x.com\r\nBcc: x@y.com", "notanemail"} {
		bad := doJSON(t, h, "POST", "/register", map[string]any{"email": badEmail, "password": "Password123!", "username": "SomeUser"})
		if bad.Code != 400 || !strings.Contains(bad.Body.String(), "Invalid email format") {
			t.Fatalf("invalid email %q should be rejected, got %d %s", badEmail, bad.Code, bad.Body.String())
		}
		if row, err := db.GetUserByEmail(ctx, badEmail); err != nil || row != nil {
			t.Fatalf("invalid email registration should not create user: row=%#v err=%v", row, err)
		}
	}
	if err := db.SetSetting(ctx, "allow_register", false); err != nil {
		t.Fatal(err)
	}
	disabled := doJSON(t, h, "POST", "/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if disabled.Code != 403 {
		t.Fatalf("disabled register should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.SetSetting(ctx, "allow_register", true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "require_invite", true); err != nil {
		t.Fatal(err)
	}
	missingInvite := doJSON(t, h, "POST", "/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if missingInvite.Code != 400 {
		t.Fatalf("missing invite should be 400, got %d", missingInvite.Code)
	}
	if err := db.CreateInvite(ctx, "VALID_CODE", 1, "once"); err != nil {
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
	second, _ := db.GetUserByEmail(ctx, "second@test.com")
	if second != nil {
		t.Fatal("overused invite should not create user")
	}
}

func TestVerificationCodeRegisterAndResetPasswordHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	ctx := context.Background()

	disabled := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if disabled.Code != 400 {
		t.Fatalf("verification disabled should be 400, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.SetSetting(ctx, "email_verify_enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "email_verify_ttl", 300); err != nil {
		t.Fatal(err)
	}

	send := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if send.Code != 200 {
		t.Fatalf("send verification status=%d body=%s", send.Code, send.Body.String())
	}
	sendBody := parseJSON(t, send)
	if sendBody["ok"] != true || sendBody["ttl"] != float64(300) {
		t.Fatalf("unexpected verification response: %#v", sendBody)
	}
	code, expiresAt, ok, err := db.GetVerificationCode(ctx, "verify@test.com", "register")
	if err != nil || !ok {
		t.Fatalf("verification code missing ok=%v err=%v", ok, err)
	}
	if len(code) != 8 || expiresAt <= database.NowMS() {
		t.Fatalf("bad verification code code=%q expires=%d", code, expiresAt)
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
	if _, _, ok, _ := db.GetVerificationCode(ctx, "verify@test.com", "register"); ok {
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
	resetCode, _, ok, err := db.GetVerificationCode(ctx, user.Email, "reset")
	if err != nil || !ok {
		t.Fatalf("reset code missing ok=%v err=%v", ok, err)
	}
	reset := doJSON(t, h, "POST", "/reset-password", map[string]any{"email": user.Email, "password": "NewPassword456!", "code": resetCode})
	if reset.Code != 200 {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	updated, _ := db.GetUserByID(ctx, user.ID)
	if !util.VerifyPassword("NewPassword456!", updated.Password) {
		t.Fatal("reset password did not update password")
	}
	reuseRefresh := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
	if reuseRefresh.Code != 401 {
		t.Fatalf("old refresh should be revoked after reset, got %d", reuseRefresh.Code)
	}
	if _, _, ok, _ := db.GetVerificationCode(ctx, user.Email, "reset"); ok {
		t.Fatal("reset verification code should be deleted after use")
	}
}

func TestMicrosoftImportProfileTokenSemantics(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "msapi@test.com", "Password123", "MsApiUser", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}

	importToken := "import-token-ok"
	httpapi.MicrosoftImportStates.Put(importToken, map[string]any{
		"user_id": user.ID,
		"kind":    "import",
		"profile": map[string]any{
			"id":   "verified_ms_id",
			"name": "VerifiedPlayer",
			"skins": []any{
				map[string]any{"url": "http://skin.url", "variant": "classic"},
			},
			"capes": []any{},
		},
	}, time.Minute)

	resp := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{
		"ms_token":     importToken,
		"profile_id":   "forged_id",
		"profile_name": "ForgedName",
		"skin_url":     "http://evil/skin.png",
	}, cookie)
	if resp.Code != 200 {
		t.Fatalf("microsoft import status=%d body=%s", resp.Code, resp.Body.String())
	}
	data := parseJSON(t, resp)
	profile := data["profile"].(map[string]any)
	if profile["id"] != "verified_ms_id" || profile["name"] != "VerifiedPlayer" {
		t.Fatalf("import should trust server-side profile only: %#v", profile)
	}
	if forged, _ := db.GetProfileByID(context.Background(), "forged_id"); forged != nil {
		t.Fatal("client-supplied forged profile id should not be persisted")
	}
	if verified, _ := db.GetProfileByID(context.Background(), "verified_ms_id"); verified == nil {
		t.Fatal("verified profile should be persisted")
	}

	replay := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": importToken}, cookie)
	if replay.Code != 400 {
		t.Fatalf("import token should be one-time, got %d body=%s", replay.Code, replay.Body.String())
	}

	otherUserToken := "import-token-other-user"
	httpapi.MicrosoftImportStates.Put(otherUserToken, map[string]any{
		"user_id": "some-other-user-id",
		"kind":    "import",
		"profile": map[string]any{"id": "x_id", "name": "X"},
	}, time.Minute)
	other := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": otherUserToken}, cookie)
	if other.Code != 403 {
		t.Fatalf("other user's token should be 403, got %d body=%s", other.Code, other.Body.String())
	}

	wrongKindToken := "import-token-wrong-kind"
	httpapi.MicrosoftImportStates.Put(wrongKindToken, map[string]any{
		"user_id": user.ID,
		"kind":    "profile",
		"profile": map[string]any{"id": "wrong_kind_id", "name": "WrongKind"},
	}, time.Minute)
	wrongKind := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": wrongKindToken}, cookie)
	if wrongKind.Code != 400 {
		t.Fatalf("wrong kind token should be 400, got %d body=%s", wrongKind.Code, wrongKind.Body.String())
	}

	missing := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": "does-not-exist"}, cookie)
	if missing.Code != 400 {
		t.Fatalf("missing token should be 400, got %d body=%s", missing.Code, missing.Body.String())
	}

	conflictToken := "import-token-uuid-conflict"
	testutil.CreateProfile(t, db, user.ID, "conflict_ms_id", "ExistingMsProfile")
	httpapi.MicrosoftImportStates.Put(conflictToken, map[string]any{
		"user_id": user.ID,
		"kind":    "import",
		"profile": map[string]any{"id": "conflict_ms_id", "name": "ConflictMsPlayer", "skins": []any{}, "capes": []any{}},
	}, time.Minute)
	conflict := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": conflictToken}, cookie)
	if conflict.Code != 400 || !strings.Contains(conflict.Body.String(), "UUID") {
		t.Fatalf("uuid conflict should be 400 with UUID detail, got %d body=%s", conflict.Code, conflict.Body.String())
	}

	nameDedupToken := "import-token-name-dedup"
	testutil.CreateProfile(t, db, user.ID, "other_ms_name", "TakenMsName")
	httpapi.MicrosoftImportStates.Put(nameDedupToken, map[string]any{
		"user_id": user.ID,
		"kind":    "import",
		"profile": map[string]any{"id": "new_ms_id", "name": "TakenMsName", "skins": []any{}, "capes": []any{}},
	}, time.Minute)
	dedup := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": nameDedupToken}, cookie)
	if dedup.Code != 200 {
		t.Fatalf("name dedup import status=%d body=%s", dedup.Code, dedup.Body.String())
	}
	dedupProfile := parseJSON(t, dedup)["profile"].(map[string]any)
	if dedupProfile["id"] != "new_ms_id" || dedupProfile["name"] != "TakenMsName_1" {
		t.Fatalf("name conflict should import with suffix: %#v", dedupProfile)
	}
	if row, _ := db.GetProfileByID(context.Background(), "new_ms_id"); row == nil || row.Name != "TakenMsName_1" {
		t.Fatalf("deduped microsoft profile not persisted: %#v", row)
	}
}

func TestMicrosoftAuthURLAndGetProfileTokenSemantics(t *testing.T) {
	userDB, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, userDB, "msflow@test.com", "Password123", "MsFlow", false)
	other := testutil.CreateUser(t, userDB, "msflow-other@test.com", "Password123", "MsFlowOther", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	otherToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, other.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}
	otherCookie := &http.Cookie{Name: "access_token", Value: otherToken}

	authURL := doJSON(t, h, "GET", "/microsoft/auth-url", nil, cookie)
	if authURL.Code != 200 {
		t.Fatalf("auth-url status=%d body=%s", authURL.Code, authURL.Body.String())
	}
	authBody := parseJSON(t, authURL)
	state, _ := authBody["state"].(string)
	if state == "" || !strings.Contains(authBody["auth_url"].(string), "state="+state) {
		t.Fatalf("unexpected auth-url body: %#v", authBody)
	}

	profileToken := "ms-profile-token"
	httpapi.MicrosoftImportStates.Put(profileToken, map[string]any{
		"user_id": user.ID,
		"kind":    "profile",
		"profile": map[string]any{
			"has_game": true,
			"profile": map[string]any{
				"id":   "ms_flow_profile",
				"name": "MsFlowPlayer",
				"skins": []any{
					map[string]any{"url": "http://skin", "variant": "slim"},
				},
				"capes": []any{},
			},
		},
	}, time.Minute)
	getProfileOther := doJSON(t, h, "POST", "/microsoft/get-profile", map[string]any{"ms_token": profileToken}, otherCookie)
	if getProfileOther.Code != 403 {
		t.Fatalf("other user's profile token should be 403, got %d body=%s", getProfileOther.Code, getProfileOther.Body.String())
	}
	// The failed cross-user attempt pops the one-shot token; seed it again for the owner path.
	httpapi.MicrosoftImportStates.Put(profileToken, map[string]any{
		"user_id": user.ID,
		"kind":    "profile",
		"profile": map[string]any{
			"has_game": true,
			"profile":  map[string]any{"id": "ms_flow_profile", "name": "MsFlowPlayer", "skins": []any{}, "capes": []any{}},
		},
	}, time.Minute)
	getProfile := doJSON(t, h, "POST", "/microsoft/get-profile", map[string]any{"ms_token": profileToken}, cookie)
	if getProfile.Code != 200 {
		t.Fatalf("get-profile status=%d body=%s", getProfile.Code, getProfile.Body.String())
	}
	getBody := parseJSON(t, getProfile)
	importToken, _ := getBody["import_token"].(string)
	if importToken == "" || getBody["has_game"] != true {
		t.Fatalf("unexpected get-profile body: %#v", getBody)
	}
	replay := doJSON(t, h, "POST", "/microsoft/get-profile", map[string]any{"ms_token": profileToken}, cookie)
	if replay.Code != 400 {
		t.Fatalf("profile token should be one-shot, got %d body=%s", replay.Code, replay.Body.String())
	}
	importResp := doJSON(t, h, "POST", "/microsoft/import-profile", map[string]any{"ms_token": importToken}, cookie)
	if importResp.Code != 200 {
		t.Fatalf("import issued token status=%d body=%s", importResp.Code, importResp.Body.String())
	}
}

func TestSettingsGroupsAndRemoteYggImportHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "settings-admin@test.com", "Password123", "SettingsAdmin", true)
	user := testutil.CreateUser(t, db, "remote@test.com", "Password123", "RemoteUser", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}

	defaults := doJSON(t, h, "GET", "/admin/settings/site", nil, adminCookie)
	if defaults.Code != 200 {
		t.Fatalf("settings defaults status=%d body=%s", defaults.Code, defaults.Body.String())
	}
	defaultBody := parseJSON(t, defaults)
	if defaultBody["site_name"] != "皮肤站" || defaultBody["allow_register"] != true || defaultBody["max_texture_size"] != float64(1024) {
		t.Fatalf("unexpected settings defaults: %#v", defaultBody)
	}
	securityDefaults := parseJSON(t, doJSON(t, h, "GET", "/admin/settings/security", nil, adminCookie))
	if securityDefaults["rate_limit_auth_attempts"] != float64(5) || securityDefaults["rate_limit_enabled"] != true {
		t.Fatalf("unexpected security defaults: %#v", securityDefaults)
	}
	emailDefaults := parseJSON(t, doJSON(t, h, "GET", "/admin/settings/email", nil, adminCookie))
	if emailDefaults["smtp_port"] != float64(465) || emailDefaults["email_verify_enabled"] != false {
		t.Fatalf("unexpected email defaults: %#v", emailDefaults)
	}
	invalidGroup := doJSON(t, h, "POST", "/admin/settings/nonsense", map[string]any{"x": 1}, adminCookie)
	if invalidGroup.Code != 400 {
		t.Fatalf("invalid settings group should be 400, got %d", invalidGroup.Code)
	}
	invalid := doJSON(t, h, "POST", "/admin/settings/site", map[string]any{"profile_uuid_mode": "bogus"}, adminCookie)
	if invalid.Code != 400 {
		t.Fatalf("invalid profile_uuid_mode should be 400, got %d", invalid.Code)
	}
	afterInvalid := parseJSON(t, doJSON(t, h, "GET", "/admin/settings/site", nil, adminCookie))
	if afterInvalid["profile_uuid_mode"] != "random" {
		t.Fatalf("invalid profile_uuid_mode should not persist: %#v", afterInvalid)
	}
	save := doJSON(t, h, "POST", "/admin/settings/site", map[string]any{"site_name": "Public Name", "allow_register": false, "max_texture_size": 2048}, adminCookie)
	if save.Code != 200 {
		t.Fatalf("save settings status=%d body=%s", save.Code, save.Body.String())
	}
	securitySave := doJSON(t, h, "POST", "/admin/settings/security", map[string]any{"rate_limit_enabled": true, "rate_limit_auth_attempts": 10}, adminCookie)
	if securitySave.Code != 200 {
		t.Fatalf("security save status=%d body=%s", securitySave.Code, securitySave.Body.String())
	}
	securitySaved := parseJSON(t, doJSON(t, h, "GET", "/admin/settings/security", nil, adminCookie))
	if securitySaved["rate_limit_auth_attempts"] != float64(10) || securitySaved["rate_limit_enabled"] != true {
		t.Fatalf("security settings did not roundtrip: %#v", securitySaved)
	}
	if err := db.SetSetting(context.Background(), "smtp_password", "secret"); err != nil {
		t.Fatal(err)
	}
	emailSave := doJSON(t, h, "POST", "/admin/settings/email", map[string]any{"smtp_host": "mail.example.com", "smtp_password": ""}, adminCookie)
	if emailSave.Code != 200 {
		t.Fatalf("email save status=%d body=%s", emailSave.Code, emailSave.Body.String())
	}
	if savedPassword, _ := db.GetSetting(context.Background(), "smtp_password", ""); savedPassword != "secret" {
		t.Fatalf("empty smtp_password should not overwrite existing secret, got %q", savedPassword)
	}
	emailSaved := parseJSON(t, doJSON(t, h, "GET", "/admin/settings/email", nil, adminCookie))
	if emailSaved["smtp_host"] != "mail.example.com" {
		t.Fatalf("email settings did not roundtrip smtp_host: %#v", emailSaved)
	}
	public := doJSON(t, h, "GET", "/public/settings", nil)
	pubBody := parseJSON(t, public)
	if pubBody["site_name"] != "Public Name" || pubBody["allow_register"] != false {
		t.Fatalf("public settings did not reflect save: %#v", pubBody)
	}
	badFallbacks := []map[string]any{
		{"fallbacks": "not-a-list"},
		{"fallbacks": []map[string]any{{"session_url": "https://s", "account_url": "", "services_url": "https://x"}}},
		{"fallbacks": []map[string]any{{"session_url": "https://s", "account_url": "https://a", "services_url": "https://x", "cache_ttl": -5}}},
	}
	for _, body := range badFallbacks {
		resp := doJSON(t, h, "POST", "/admin/settings/fallback", body, adminCookie)
		if resp.Code != 400 {
			t.Fatalf("bad fallback settings should be 400, got %d body=%s for %#v", resp.Code, resp.Body.String(), body)
		}
	}
	fallbackSave := doJSON(t, h, "POST", "/admin/settings/fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []map[string]any{{
			"priority":         1,
			"session_url":      "https://session.example",
			"account_url":      "https://account.example",
			"services_url":     "https://services.example",
			"cache_ttl":        "30",
			"skin_domains":     []any{"skins.example", " cdn.example ", ""},
			"enable_profile":   true,
			"enable_hasjoined": true,
			"enable_whitelist": false,
			"note":             "primary",
		}},
	}, adminCookie)
	if fallbackSave.Code != 200 {
		t.Fatalf("fallback save status=%d body=%s", fallbackSave.Code, fallbackSave.Body.String())
	}
	fallbackGroup := doJSON(t, h, "GET", "/admin/settings/fallback", nil, adminCookie)
	if fallbackGroup.Code != 200 {
		t.Fatalf("fallback get status=%d body=%s", fallbackGroup.Code, fallbackGroup.Body.String())
	}
	fallbackBody := parseJSON(t, fallbackGroup)
	if fallbackBody["fallback_strategy"] != "parallel" || len(fallbackBody["fallbacks"].([]any)) != 1 {
		t.Fatalf("unexpected fallback settings: %#v", fallbackBody)
	}
	publicAfterFallback := parseJSON(t, doJSON(t, h, "GET", "/public/settings", nil))
	status := publicAfterFallback["mojang_status_urls"].(map[string]any)
	if status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" {
		t.Fatalf("public settings should reflect fallback endpoint: %#v", status)
	}

	eps, err := db.ListFallbackEndpoints(context.Background())
	if err != nil || len(eps) != 1 {
		t.Fatalf("fallback endpoint not persisted: %#v err=%v", eps, err)
	}
	endpointID := eps[0]["id"].(int)
	addWL := doJSON(t, h, "POST", "/admin/official-whitelist", map[string]any{"username": "Whitelisted", "endpoint_id": endpointID}, adminCookie)
	if addWL.Code != 200 {
		t.Fatalf("add whitelist status=%d body=%s", addWL.Code, addWL.Body.String())
	}
	listWL := doJSON(t, h, "GET", "/admin/official-whitelist?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if listWL.Code != 200 || len(parseJSON(t, listWL)["items"].([]any)) != 1 {
		t.Fatalf("list whitelist unexpected: %d %s", listWL.Code, listWL.Body.String())
	}
	removeWL := doJSON(t, h, "DELETE", "/admin/official-whitelist/Whitelisted?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if removeWL.Code != 200 {
		t.Fatalf("remove whitelist status=%d body=%s", removeWL.Code, removeWL.Body.String())
	}
	listWL = doJSON(t, h, "GET", "/admin/official-whitelist?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if len(parseJSON(t, listWL)["items"].([]any)) != 0 {
		t.Fatalf("whitelist should be empty: %s", listWL.Body.String())
	}

	carouselEmpty := doJSON(t, h, "GET", "/public/carousel", nil)
	if carouselEmpty.Code != 200 || carouselEmpty.Body.String() == "" {
		t.Fatalf("public carousel should return an array: %d %s", carouselEmpty.Code, carouselEmpty.Body.String())
	}
	carouselUpload := doMultipart(t, h, "POST", "/admin/carousel", nil, "file", "banner.png", pngTexture(t, 64, 64), adminCookie)
	if carouselUpload.Code != 200 {
		t.Fatalf("carousel upload status=%d body=%s", carouselUpload.Code, carouselUpload.Body.String())
	}
	filename := parseJSON(t, carouselUpload)["filename"].(string)
	carouselList := doJSON(t, h, "GET", "/public/carousel", nil)
	if !strings.Contains(carouselList.Body.String(), filename) {
		t.Fatalf("carousel list missing uploaded filename %q: %s", filename, carouselList.Body.String())
	}
	carouselDelete := doJSON(t, h, "DELETE", "/admin/carousel/"+filename, nil, adminCookie)
	if carouselDelete.Code != 200 {
		t.Fatalf("carousel delete status=%d body=%s", carouselDelete.Code, carouselDelete.Body.String())
	}

	getProfiles := doJSON(t, h, "POST", "/remote-ygg/get-profiles", map[string]any{"profiles": []any{map[string]any{"id": "remote_1", "name": "RemoteOne"}}}, userCookie)
	if getProfiles.Code != 200 || len(parseJSON(t, getProfiles)["profiles"].([]any)) != 1 {
		t.Fatalf("remote get-profiles unexpected: %d %s", getProfiles.Code, getProfiles.Body.String())
	}
	testutil.CreateProfile(t, db, user.ID, "existing_remote_name", "RemotePlayer")
	single := doJSON(t, h, "POST", "/remote-ygg/import-profile", map[string]any{"profile_id": "remote_single", "profile_name": "RemotePlayer"}, userCookie)
	if single.Code != 200 {
		t.Fatalf("single remote import status=%d body=%s", single.Code, single.Body.String())
	}
	singleBody := parseJSON(t, single)
	if singleBody["id"] != "remote_single" || singleBody["name"] != "RemotePlayer_1" {
		t.Fatalf("unexpected single import response: %#v", singleBody)
	}
	singleProfile, _ := db.GetProfileByID(context.Background(), "remote_single")
	if singleProfile == nil || singleProfile.Name != "RemotePlayer_1" {
		t.Fatalf("single import not persisted: %#v", singleProfile)
	}
	notList := doJSON(t, h, "POST", "/remote-ygg/import-profiles", map[string]any{"profiles": "bad"}, userCookie)
	if notList.Code != 400 {
		t.Fatalf("non-list profiles should be 400, got %d", notList.Code)
	}
	emptyList := doJSON(t, h, "POST", "/remote-ygg/import-profiles", map[string]any{"profiles": []any{}}, userCookie)
	if emptyList.Code != 400 {
		t.Fatalf("empty profiles should be 400, got %d", emptyList.Code)
	}
	imported := doJSON(t, h, "POST", "/remote-ygg/import-profiles", map[string]any{"profiles": []map[string]string{
		{"profile_id": "remote_pid_1", "profile_name": "RemotePlayer1"},
		{"profile_id": "remote_pid_2", "profile_name": "RemotePlayer2"},
		{"profile_id": "", "profile_name": "BrokenProfile"},
	}}, userCookie)
	if imported.Code != 200 {
		t.Fatalf("remote import status=%d body=%s", imported.Code, imported.Body.String())
	}
	importBody := parseJSON(t, imported)
	if importBody["success_count"] != float64(2) || importBody["failure_count"] != float64(1) {
		t.Fatalf("unexpected remote import result: %#v", importBody)
	}
	p1, _ := db.GetProfileByID(context.Background(), "remote_pid_1")
	p2, _ := db.GetProfileByID(context.Background(), "remote_pid_2")
	if p1 == nil || p2 == nil || p1.Name != "RemotePlayer1" || p2.Name != "RemotePlayer2" {
		t.Fatalf("remote profiles not persisted: %#v %#v", p1, p2)
	}
}

func TestYggdrasilFallbackRoutes(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	ctx := context.Background()

	var seen []string
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/session/minecraft/hasJoined":
			if r.URL.Query().Get("username") != "RemotePlayer" || r.URL.Query().Get("serverId") != "remote-server" || r.URL.Query().Get("ip") != "127.0.0.1" {
				t.Fatalf("unexpected hasJoined query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"remoteid","name":"RemotePlayer"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/session/minecraft/profile/remoteid":
			if r.URL.Query().Get("unsigned") != "false" {
				t.Fatalf("expected unsigned=false, got %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"remoteid","name":"RemotePlayer","properties":[{"name":"textures","value":"v"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/profiles/minecraft/RemoteAccount":
			_, _ = w.Write([]byte(`{"id":"accountid","name":"RemoteAccount"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/profiles/minecraft":
			var names []string
			if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
				t.Fatalf("decode fallback bulk body: %v", err)
			}
			if len(names) != 1 || names[0] != "RemoteBulk" {
				t.Fatalf("unexpected fallback bulk names: %#v", names)
			}
			_, _ = w.Write([]byte(`[{"id":"bulkid","name":"RemoteBulk"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/minecraft/profile/lookup/name/RemoteServices":
			_, _ = w.Write([]byte(`{"id":"servicesid","name":"RemoteServices"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fallback.Close()

	if err := db.SaveFallbackEndpoints(ctx, []database.FallbackEndpoint{{
		Priority: 1, SessionURL: fallback.URL, AccountURL: fallback.URL, ServicesURL: fallback.URL,
		CacheTTL: 60, EnableProfile: true, EnableHasJoined: true,
	}}); err != nil {
		t.Fatal(err)
	}

	hasJoined := doJSON(t, h, "GET", "/sessionserver/session/minecraft/hasJoined?username=RemotePlayer&serverId=remote-server&ip=127.0.0.1", nil)
	if hasJoined.Code != 200 || !strings.Contains(hasJoined.Body.String(), `"RemotePlayer"`) {
		t.Fatalf("hasJoined fallback failed: %d %s", hasJoined.Code, hasJoined.Body.String())
	}

	profile := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/remoteid?unsigned=false", nil)
	if profile.Code != 200 || !strings.Contains(profile.Body.String(), `"properties"`) {
		t.Fatalf("profile fallback failed: %d %s", profile.Code, profile.Body.String())
	}

	account := doJSON(t, h, "GET", "/api/profiles/minecraft/RemoteAccount", nil)
	if account.Code != 200 || parseJSON(t, account)["id"] != "accountid" {
		t.Fatalf("account lookup fallback failed: %d %s", account.Code, account.Body.String())
	}

	userAccount := doJSON(t, h, "GET", "/users/profiles/minecraft/RemoteAccount", nil)
	if userAccount.Code != 200 || parseJSON(t, userAccount)["id"] != "accountid" {
		t.Fatalf("users profile fallback failed: %d %s", userAccount.Code, userAccount.Body.String())
	}

	bulk := doJSON(t, h, "POST", "/api/profiles/minecraft", []string{"RemoteBulk"})
	var bulkBody []map[string]any
	if err := json.Unmarshal(bulk.Body.Bytes(), &bulkBody); err != nil {
		t.Fatalf("decode bulk body: %v body=%s", err, bulk.Body.String())
	}
	if bulk.Code != 200 || len(bulkBody) != 1 || bulkBody[0]["id"] != "bulkid" {
		t.Fatalf("bulk fallback failed: %d %s", bulk.Code, bulk.Body.String())
	}

	services := doJSON(t, h, "GET", "/minecraft/profile/lookup/name/RemoteServices", nil)
	if services.Code != 200 || parseJSON(t, services)["id"] != "servicesid" {
		t.Fatalf("services lookup fallback failed: %d %s", services.Code, services.Body.String())
	}

	if len(seen) < 6 {
		t.Fatalf("expected fallback server to receive all lookup requests, saw %#v", seen)
	}
}
