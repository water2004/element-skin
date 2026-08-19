package yggdrasil_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
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

func TestLookupNamesMergesLocalAndFallbackProfilesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	user := testutil.CreateUser(t, db, "ygg-lookup-merge@test.com", "Password123", "YggLookupMerge", false)
	local := testutil.CreateProfile(t, db, user.ID, "ygg_lookup_merge_local", "LocalPlayer")

	fallbackCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fallbackCalls++
		if req.Method != http.MethodPost || req.URL.Path != "/profiles/minecraft" {
			t.Fatalf("fallback request=%s %s; want POST /profiles/minecraft", req.Method, req.URL.Path)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("fallback content type=%q; want application/json", req.Header.Get("Content-Type"))
		}
		var names []string
		if err := json.NewDecoder(req.Body).Decode(&names); err != nil {
			t.Fatal(err)
		}
		if len(names) != 2 || names[0] != "RemoteOne" || names[1] != "remoteTwo" {
			t.Fatalf("fallback names=%#v; want only exact missing local names", names)
		}
		_, _ = w.Write([]byte(`[{"id":"remote_one_id","name":"RemoteOne"},{"id":"remote_two_id","name":"remoteTwo"}]`))
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(t.Context(), []dbfallback.Endpoint{{
		Priority:      1,
		SessionURL:    server.URL,
		AccountURL:    server.URL,
		ServicesURL:   server.URL,
		CacheTTL:      60,
		EnableProfile: true,
	}}); err != nil {
		t.Fatal(err)
	}
	h := yggdrasil.NewWithHTTPClient(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg}, server.Client())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/profiles/minecraft",
		strings.NewReader(`["RemoteOne","LocalPlayer","remoteTwo","LOCALPLAYER"]`),
	)
	rec := httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("merged lookup status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 3 ||
		body[0]["id"] != local.ID ||
		body[0]["name"] != local.Name ||
		body[1]["id"] != "remote_one_id" ||
		body[1]["name"] != "RemoteOne" ||
		body[2]["id"] != "remote_two_id" ||
		body[2]["name"] != "remoteTwo" ||
		fallbackCalls != 1 {
		t.Fatalf("merged lookup body=%#v calls=%d; want exact local then fallback results", body, fallbackCalls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`[]`))
	rec = httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" || fallbackCalls != 1 {
		t.Fatalf("empty lookup status=%d body=%q calls=%d; want [] and no fallback", rec.Code, rec.Body.String(), fallbackCalls)
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
	descriptor, ok := body["error"].(map[string]any)
	if !ok || len(descriptor) != 3 || descriptor["object"] != "request" ||
		descriptor["operation"] != "decode" || descriptor["reason"] != "invalid" {
		t.Fatalf("bad bulk lookup body mismatch: %#v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`[`))
	rec = httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
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

func TestLookupRoutesWriteExactFallbackResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/session/minecraft/hasJoined":
			if req.URL.Query().Get("username") != "RemoteJoin" ||
				req.URL.Query().Get("serverId") != "remote-server" ||
				req.URL.Query().Get("ip") != "127.0.0.2" {
				t.Fatalf("hasJoined fallback query mismatch: %s", req.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"remote_join_id","name":"RemoteJoin"}`))
		case req.Method == http.MethodGet && req.URL.Path == "/session/minecraft/profile/remote-profile":
			if req.URL.Query().Get("unsigned") != "false" {
				t.Fatalf("profile fallback unsigned=%q, want false", req.URL.Query().Get("unsigned"))
			}
			_, _ = w.Write([]byte(`{"id":"remote-profile","name":"RemoteProfile"}`))
		case req.Method == http.MethodGet && req.URL.Path == "/users/profiles/minecraft/RemoteName":
			_, _ = w.Write([]byte(`{"id":"remote_name_id","name":"RemoteName"}`))
		case req.Method == http.MethodGet && req.URL.Path == "/minecraft/profile/lookup/name/RemoteServices":
			_, _ = w.Write([]byte(`{"id":"remote_services_id","name":"RemoteServices"}`))
		default:
			t.Fatalf("unexpected fallback request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(t.Context(), []dbfallback.Endpoint{{
		Priority:        1,
		SessionURL:      server.URL,
		AccountURL:      server.URL,
		ServicesURL:     server.URL,
		CacheTTL:        60,
		EnableHasJoined: true,
		EnableProfile:   true,
	}}); err != nil {
		t.Fatal(err)
	}
	h := yggdrasil.NewWithHTTPClient(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg}, server.Client())

	req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/hasJoined?username=RemoteJoin&serverId=remote-server&ip=127.0.0.2", nil)
	rec := httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"id":"remote_join_id","name":"RemoteJoin"}` {
		t.Fatalf("fallback hasJoined response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/remote-profile?unsigned=false", nil)
	req.SetPathValue("uuid", "remote-profile")
	rec = httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"id":"remote-profile","name":"RemoteProfile"}` {
		t.Fatalf("fallback profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/minecraft/RemoteName", nil)
	req.SetPathValue("playerName", "RemoteName")
	rec = httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"id":"remote_name_id","name":"RemoteName"}` {
		t.Fatalf("fallback account lookup response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/minecraft/profile/lookup/name/RemoteServices", nil)
	req.SetPathValue("playerName", "RemoteServices")
	rec = httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"id":"remote_services_id","name":"RemoteServices"}` {
		t.Fatalf("fallback services lookup response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(requests) != 4 {
		t.Fatalf("fallback requests=%#v; want four exact fallback calls", requests)
	}
}

func TestLookupRoutesReturnExactDependencyErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*yggdrasil.Handler, *httptest.ResponseRecorder)
	}{
		{
			name: "hasJoined",
			call: func(h *yggdrasil.Handler, rec *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/hasJoined?username=Player&serverId=server", nil)
				h.HasJoined(rec, req)
			},
		},
		{
			name: "profile",
			call: func(h *yggdrasil.Handler, rec *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/profile-id", nil)
				req.SetPathValue("uuid", "profile-id")
				h.Profile(rec, req)
			},
		},
		{
			name: "lookupName",
			call: func(h *yggdrasil.Handler, rec *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodGet, "/api/profiles/minecraft/Player", nil)
				req.SetPathValue("playerName", "Player")
				h.LookupName(rec, req)
			},
		},
		{
			name: "lookupNames",
			call: func(h *yggdrasil.Handler, rec *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`["Player"]`))
				h.LookupNames(rec, req)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			cfg := testutil.TestConfig()
			redis := testutil.NewMemoryRedis()
			h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
			db.Close()
			rec := httptest.NewRecorder()
			tc.call(&h, rec)
			if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
				t.Fatalf("%s dependency error mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
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
