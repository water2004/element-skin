package integration_test

import (
	"context"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSettingsGroupsAndRemoteYggImportHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "settings-admin@test.com", "Password123", "SettingsAdmin", true)
	user := testutil.CreateUser(t, db, "remote@test.com", "Password123", "RemoteUser", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}

	defaults := doJSON(t, h, "GET", "/v1/admin/settings/site", nil, adminCookie)
	if defaults.Code != 200 {
		t.Fatalf("settings defaults status=%d body=%s", defaults.Code, defaults.Body.String())
	}
	defaultBody := parseJSON(t, defaults)
	if defaultBody["site_name"] != "皮肤站" || defaultBody["allow_register"] != true || defaultBody["max_texture_size"] != float64(1024) {
		t.Fatalf("unexpected settings defaults: %#v", defaultBody)
	}
	securityDefaults := parseJSON(t, doJSON(t, h, "GET", "/v1/admin/settings/security", nil, adminCookie))
	if securityDefaults["rate_limit_auth_attempts"] != float64(5) || securityDefaults["rate_limit_enabled"] != true {
		t.Fatalf("unexpected security defaults: %#v", securityDefaults)
	}
	emailDefaults := parseJSON(t, doJSON(t, h, "GET", "/v1/admin/settings/email", nil, adminCookie))
	if emailDefaults["smtp_port"] != float64(465) || emailDefaults["email_verify_enabled"] != false {
		t.Fatalf("unexpected email defaults: %#v", emailDefaults)
	}
	invalidGroup := doJSON(t, h, "POST", "/v1/admin/settings/nonsense", map[string]any{"x": 1}, adminCookie)
	if invalidGroup.Code != 400 {
		t.Fatalf("invalid settings group should be 400, got %d", invalidGroup.Code)
	}
	invalid := doJSON(t, h, "POST", "/v1/admin/settings/site", map[string]any{"profile_uuid_mode": "bogus"}, adminCookie)
	if invalid.Code != 400 {
		t.Fatalf("invalid profile_uuid_mode should be 400, got %d", invalid.Code)
	}
	afterInvalid := parseJSON(t, doJSON(t, h, "GET", "/v1/admin/settings/site", nil, adminCookie))
	if afterInvalid["profile_uuid_mode"] != "random" {
		t.Fatalf("invalid profile_uuid_mode should not persist: %#v", afterInvalid)
	}
	save := doJSON(t, h, "POST", "/v1/admin/settings/site", map[string]any{"site_name": "Public Name", "allow_register": false, "max_texture_size": 2048}, adminCookie)
	if save.Code != 200 {
		t.Fatalf("save settings status=%d body=%s", save.Code, save.Body.String())
	}
	securitySave := doJSON(t, h, "POST", "/v1/admin/settings/security", map[string]any{"rate_limit_enabled": true, "rate_limit_auth_attempts": 10}, adminCookie)
	if securitySave.Code != 200 {
		t.Fatalf("security save status=%d body=%s", securitySave.Code, securitySave.Body.String())
	}
	securitySaved := parseJSON(t, doJSON(t, h, "GET", "/v1/admin/settings/security", nil, adminCookie))
	if securitySaved["rate_limit_auth_attempts"] != float64(10) || securitySaved["rate_limit_enabled"] != true {
		t.Fatalf("security settings did not roundtrip: %#v", securitySaved)
	}
	if err := db.Settings.Set(context.Background(), "smtp_password", "secret"); err != nil {
		t.Fatal(err)
	}
	emailSave := doJSON(t, h, "POST", "/v1/admin/settings/email", map[string]any{"smtp_host": "mail.example.com", "smtp_password": ""}, adminCookie)
	if emailSave.Code != 200 {
		t.Fatalf("email save status=%d body=%s", emailSave.Code, emailSave.Body.String())
	}
	if savedPassword, _ := db.Settings.Get(context.Background(), "smtp_password", ""); savedPassword != "secret" {
		t.Fatalf("empty smtp_password should not overwrite existing secret, got %q", savedPassword)
	}
	emailSaved := parseJSON(t, doJSON(t, h, "GET", "/v1/admin/settings/email", nil, adminCookie))
	if emailSaved["smtp_host"] != "mail.example.com" {
		t.Fatalf("email settings did not roundtrip smtp_host: %#v", emailSaved)
	}
	public := doJSON(t, h, "GET", "/v1/public/settings", nil)
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
		resp := doJSON(t, h, "POST", "/v1/admin/settings/fallback", body, adminCookie)
		if resp.Code != 400 {
			t.Fatalf("bad fallback settings should be 400, got %d body=%s for %#v", resp.Code, resp.Body.String(), body)
		}
	}
	fallbackSave := doJSON(t, h, "POST", "/v1/admin/settings/fallback", map[string]any{
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
	fallbackGroup := doJSON(t, h, "GET", "/v1/admin/settings/fallback", nil, adminCookie)
	if fallbackGroup.Code != 200 {
		t.Fatalf("fallback get status=%d body=%s", fallbackGroup.Code, fallbackGroup.Body.String())
	}
	fallbackBody := parseJSON(t, fallbackGroup)
	if fallbackBody["fallback_strategy"] != "parallel" || len(fallbackBody["fallbacks"].([]any)) != 1 {
		t.Fatalf("unexpected fallback settings: %#v", fallbackBody)
	}
	publicAfterFallback := parseJSON(t, doJSON(t, h, "GET", "/v1/public/settings", nil))
	status := publicAfterFallback["mojang_status_urls"].(map[string]any)
	if status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" {
		t.Fatalf("public settings should reflect fallback endpoint: %#v", status)
	}

	eps, err := db.Fallbacks.ListEndpoints(context.Background())
	if err != nil || len(eps) != 1 {
		t.Fatalf("fallback endpoint not persisted: %#v err=%v", eps, err)
	}
	endpointID := eps[0]["id"].(int)
	addWL := doJSON(t, h, "POST", "/v1/admin/official-whitelist", map[string]any{"username": "Whitelisted", "endpoint_id": endpointID}, adminCookie)
	if addWL.Code != 200 {
		t.Fatalf("add whitelist status=%d body=%s", addWL.Code, addWL.Body.String())
	}
	listWL := doJSON(t, h, "GET", "/v1/admin/official-whitelist?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if listWL.Code != 200 || len(parseJSON(t, listWL)["items"].([]any)) != 1 {
		t.Fatalf("list whitelist unexpected: %d %s", listWL.Code, listWL.Body.String())
	}
	removeWL := doJSON(t, h, "DELETE", "/v1/admin/official-whitelist/Whitelisted?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if removeWL.Code != 200 {
		t.Fatalf("remove whitelist status=%d body=%s", removeWL.Code, removeWL.Body.String())
	}
	listWL = doJSON(t, h, "GET", "/v1/admin/official-whitelist?endpoint_id="+strconv.Itoa(endpointID), nil, adminCookie)
	if len(parseJSON(t, listWL)["items"].([]any)) != 0 {
		t.Fatalf("whitelist should be empty: %s", listWL.Body.String())
	}

	homepageEmpty := doJSON(t, h, "GET", "/v1/public/homepage-media", nil)
	if homepageEmpty.Code != 200 || homepageEmpty.Body.String() == "" {
		t.Fatalf("public homepage media should return an array: %d %s", homepageEmpty.Code, homepageEmpty.Body.String())
	}
	homepageUpload := doMultipart(t, h, "POST", "/v1/admin/homepage-media/image", nil, "file", "banner.png", pngTexture(t, 64, 64), adminCookie)
	if homepageUpload.Code != 200 {
		t.Fatalf("homepage media upload status=%d body=%s", homepageUpload.Code, homepageUpload.Body.String())
	}
	uploadedMedia := parseJSON(t, homepageUpload)
	mediaID := uploadedMedia["id"].(string)
	storagePath := uploadedMedia["storage_path"].(string)
	homepageList := doJSON(t, h, "GET", "/v1/public/homepage-media", nil)
	if !strings.Contains(homepageList.Body.String(), storagePath) {
		t.Fatalf("homepage media list missing uploaded storage path %q: %s", storagePath, homepageList.Body.String())
	}
	homepageDelete := doJSON(t, h, "DELETE", "/v1/admin/homepage-media/"+mediaID, nil, adminCookie)
	if homepageDelete.Code != 200 {
		t.Fatalf("homepage media delete status=%d body=%s", homepageDelete.Code, homepageDelete.Body.String())
	}

	getProfiles := doJSON(t, h, "POST", "/v1/imports/remote-ygg/profiles/preview", map[string]any{"profiles": []any{map[string]any{"id": "remote_1", "name": "RemoteOne"}}}, userCookie)
	if getProfiles.Code != 200 || len(parseJSON(t, getProfiles)["profiles"].([]any)) != 1 {
		t.Fatalf("remote get-profiles unexpected: %d %s", getProfiles.Code, getProfiles.Body.String())
	}
	testutil.CreateProfile(t, db, user.ID, "existing_remote_name", "RemotePlayer")
	single := doJSON(t, h, "POST", "/v1/imports/remote-ygg/profiles/import", map[string]any{"profile_id": "remote_single", "profile_name": "RemotePlayer"}, userCookie)
	if single.Code != 200 {
		t.Fatalf("single remote import status=%d body=%s", single.Code, single.Body.String())
	}
	singleBody := parseJSON(t, single)
	if singleBody["id"] != "remote_single" || singleBody["name"] != "RemotePlayer_1" {
		t.Fatalf("unexpected single import response: %#v", singleBody)
	}
	singleProfile, _ := db.Profiles.GetByID(context.Background(), "remote_single")
	if singleProfile == nil || singleProfile.Name != "RemotePlayer_1" {
		t.Fatalf("single import not persisted: %#v", singleProfile)
	}
	notList := doJSON(t, h, "POST", "/v1/imports/remote-ygg/profiles/import-batch", map[string]any{"profiles": "bad"}, userCookie)
	if notList.Code != 400 {
		t.Fatalf("non-list profiles should be 400, got %d", notList.Code)
	}
	emptyList := doJSON(t, h, "POST", "/v1/imports/remote-ygg/profiles/import-batch", map[string]any{"profiles": []any{}}, userCookie)
	if emptyList.Code != 400 {
		t.Fatalf("empty profiles should be 400, got %d", emptyList.Code)
	}
	imported := doJSON(t, h, "POST", "/v1/imports/remote-ygg/profiles/import-batch", map[string]any{"profiles": []map[string]string{
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
	p1, _ := db.Profiles.GetByID(context.Background(), "remote_pid_1")
	p2, _ := db.Profiles.GetByID(context.Background(), "remote_pid_2")
	if p1 == nil || p2 == nil || p1.Name != "RemotePlayer1" || p2.Name != "RemotePlayer2" {
		t.Fatalf("remote profiles not persisted: %#v %#v", p1, p2)
	}
}
