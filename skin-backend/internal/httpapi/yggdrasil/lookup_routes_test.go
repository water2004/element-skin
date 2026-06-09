package yggdrasil_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/yggdrasil"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestLookupRoutesNamesReturnExactLocalProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-lookup@test.com", "Password123", "YggLookup", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_lookup_profile", "YggLookupProfile")

	req := httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`["YggLookupProfile","MissingName"]`))
	rec := httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup names response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode lookup names response: %v body=%q", err, rec.Body.String())
	}
	if len(body) != 1 || body[0]["id"] != profile.ID || body[0]["name"] != "YggLookupProfile" {
		t.Fatalf("lookup names should include only existing profiles exactly: %#v", body)
	}
}

func TestProfileAndLookupRoutesReturnLocalProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-profile-route@test.com", "Password123", "YggProfileRoute", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_route", "YggProfileRoutePlayer")

	req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/"+profile.ID+"?unsigned=true", nil)
	req.SetPathValue("uuid", profile.ID)
	rec := httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile route status=%d body=%q", rec.Code, rec.Body.String())
	}
	var profileBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profileBody); err != nil {
		t.Fatal(err)
	}
	if profileBody["id"] != profile.ID || profileBody["name"] != profile.Name {
		t.Fatalf("profile route body mismatch: %#v", profileBody)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/minecraft/"+profile.Name, nil)
	req.SetPathValue("playerName", profile.Name)
	rec = httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) {
		t.Fatalf("lookup name route mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestProfileRouteUnsignedQueryControlsSignatureExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-profile-sign@test.com", "Password123", "YggProfileSign", false)
	profile := testutil.CreateProfile(t, db, user.ID, "11111111222233334444555555555555", "YggProfileSignPlayer")
	dashedID := "11111111-2222-3333-4444-555555555555"

	req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/"+dashedID+"?unsigned=false", nil)
	req.SetPathValue("uuid", dashedID)
	rec := httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed profile status=%d body=%q", rec.Code, rec.Body.String())
	}
	signedTexture := firstTextureProperty(t, rec.Body.Bytes())
	if signedTexture["signature"] == "" {
		t.Fatalf("unsigned=false should include a textures signature: %#v", signedTexture)
	}
	assertTexturePayloadProfile(t, signedTexture["value"].(string), profile.ID, profile.Name)

	req = httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/"+profile.ID+"?unsigned=true", nil)
	req.SetPathValue("uuid", profile.ID)
	rec = httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unsigned profile status=%d body=%q", rec.Code, rec.Body.String())
	}
	unsignedTexture := firstTextureProperty(t, rec.Body.Bytes())
	if _, ok := unsignedTexture["signature"]; ok {
		t.Fatalf("unsigned=true should omit textures signature: %#v", unsignedTexture)
	}

	req = httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/missing-profile?unsigned=false", nil)
	req.SetPathValue("uuid", "missing-profile")
	rec = httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("missing profile should be exact 204 with empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestLookupRoutesProtocolMissesAndBadBulkBodyExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/api/profiles/minecraft/MissingPlayer", nil)
	req.SetPathValue("playerName", "MissingPlayer")
	rec := httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("lookup name miss should be exact 204 with empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`{"name":"not-an-array"}`))
	rec = httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad bulk lookup body status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bad bulk lookup body: %v body=%q", err, rec.Body.String())
	}
	if body["detail"] != "Request body must be an array" {
		t.Fatalf("bad bulk lookup body mismatch: %#v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`[`))
	rec = httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Request body must be an array\"}\n" {
		t.Fatalf("malformed bulk lookup body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestLookupRoutesFallbackMissesReturnExactNoContent(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/hasJoined?username=MissingPlayer&serverId=missing-server", nil)
	rec := httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("hasJoined local+fallback miss should be exact 204 empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/missing-profile?unsigned=false", nil)
	req.SetPathValue("uuid", "missing-profile")
	rec = httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("profile local+fallback miss should be exact 204 empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/minecraft/profile/lookup/name/MissingServices", nil)
	req.SetPathValue("playerName", "MissingServices")
	rec = httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("services lookup local+fallback miss should be exact 204 empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWriteFallbackForTest(t *testing.T) {
	rec := httptest.NewRecorder()
	if !yggdrasil.WriteFallbackForTest(rec, &fallbacksvc.FallbackResponse{Status: http.StatusAccepted, Body: []byte(`{"ok":true}`)}) {
		t.Fatal("fallback response should be written")
	}
	if rec.Code != http.StatusAccepted || rec.Body.String() != `{"ok":true}` {
		t.Fatalf("fallback response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if yggdrasil.WriteFallbackForTest(httptest.NewRecorder(), nil) {
		t.Fatal("nil fallback response should not be written")
	}
}

func firstTextureProperty(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var profile map[string]any
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("decode profile response: %v body=%q", err, string(body))
	}
	props, ok := profile["properties"].([]any)
	if !ok || len(props) == 0 {
		t.Fatalf("profile properties missing: %#v", profile)
	}
	texture, ok := props[0].(map[string]any)
	if !ok || texture["name"] != "textures" || texture["value"] == "" {
		t.Fatalf("first profile property should be textures: %#v", props[0])
	}
	return texture
}

func assertTexturePayloadProfile(t *testing.T, encoded, id, name string) {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode textures payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("unmarshal textures payload: %v payload=%q", err, string(decoded))
	}
	if payload["profileId"] != id || payload["profileName"] != name {
		t.Fatalf("textures payload profile mismatch: got=%#v want id=%q name=%q", payload, id, name)
	}
}
