package minecraft_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/minecraft"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestMinecraftRoutesReturnExactProfileAndTextureResponses(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	user := testutil.CreateUser(t, db, "minecraft-route@test.com", "pw", "MinecraftRoute", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_route_profile", "MinecraftRoute")
	skin := "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if err := db.Profiles.UpdateSkin(t.Context(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}

	req := minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/by-name/"+profile.Name, nil)
	req.SetPathValue("name", profile.Name)
	rec := httptest.NewRecorder()
	h.ProfileByName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile by name status=%d body=%q", rec.Code, rec.Body.String())
	}
	var byName map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &byName); err != nil {
		t.Fatal(err)
	}
	if byName["id"] != profile.ID || byName["name"] != profile.Name || byName["owner_user_id"] != user.ID || byName["public"] != true {
		t.Fatalf("profile by name body mismatch: %#v", byName)
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/"+profile.ID+"/textures-property", nil)
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.TexturesProperty(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("textures property status=%d body=%q", rec.Code, rec.Body.String())
	}
	var textureBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &textureBody); err != nil {
		t.Fatal(err)
	}
	property := textureBody["textures_property"].(map[string]any)
	if textureBody["profile_id"] != profile.ID || property["name"] != "textures" || property["value"] == "" || property["signature"] == "" {
		t.Fatalf("textures property body mismatch: %#v", textureBody)
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/by-name/"+profile.Name, nil)
	req.SetPathValue("path", "by-name/"+profile.Name)
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profiles dispatcher by-name status=%d body=%q", rec.Code, rec.Body.String())
	}
	var dispatchedByName map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dispatchedByName); err != nil {
		t.Fatal(err)
	}
	if dispatchedByName["id"] != profile.ID || dispatchedByName["name"] != profile.Name {
		t.Fatalf("profiles dispatcher by-name mismatch: %#v", dispatchedByName)
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/"+profile.ID, nil)
	req.SetPathValue("path", profile.ID)
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profiles dispatcher by-id status=%d body=%q", rec.Code, rec.Body.String())
	}
	var dispatchedByID map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dispatchedByID); err != nil {
		t.Fatal(err)
	}
	if dispatchedByID["id"] != profile.ID || dispatchedByID["name"] != profile.Name {
		t.Fatalf("profiles dispatcher by-id mismatch: %#v", dispatchedByID)
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/"+profile.ID+"/textures-property", nil)
	req.SetPathValue("path", profile.ID+"/textures-property")
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profiles dispatcher textures status=%d body=%q", rec.Code, rec.Body.String())
	}
	var dispatchedTexture map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dispatchedTexture); err != nil {
		t.Fatal(err)
	}
	if dispatchedTexture["profile_id"] != profile.ID {
		t.Fatalf("profiles dispatcher textures mismatch: %#v", dispatchedTexture)
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/", nil)
	req.SetPathValue("path", "")
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_route\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("profiles dispatcher not found mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMinecraftRoutesValidateBulkBodyAndMissingProfilesExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})

	req := minecraftPublicRequest(http.MethodPost, "/v2/minecraft/profiles/by-names", strings.NewReader(`{"names":["Missing"]}`))
	rec := httptest.NewRecorder()
	h.ProfilesByNames(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("missing bulk profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = minecraftPublicRequest(http.MethodPost, "/v2/minecraft/profiles/by-names", strings.NewReader(`[`))
	rec = httptest.NewRecorder()
	h.ProfilesByNames(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid bulk json response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/missing-profile", nil)
	req.SetPathValue("profile_id", "missing-profile")
	rec = httptest.NewRecorder()
	h.ProfileByID(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/by-name/Missing", nil)
	req.SetPathValue("name", "Missing")
	rec = httptest.NewRecorder()
	h.ProfileByName(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing profile by name mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = minecraftPublicRequest(http.MethodGet, "/v2/minecraft/profiles/missing-profile/textures-property", nil)
	req.SetPathValue("profile_id", "missing-profile")
	rec = httptest.NewRecorder()
	h.TexturesProperty(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing textures property mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	names := make([]string, 101)
	for i := range names {
		names[i] = "Player"
	}
	body, err := json.Marshal(map[string]any{"names": names})
	if err != nil {
		t.Fatal(err)
	}
	req = minecraftPublicRequest(http.MethodPost, "/v2/minecraft/profiles/by-names", strings.NewReader(string(body)))
	rec = httptest.NewRecorder()
	h.ProfilesByNames(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"read\",\"reason\":\"exceeded\"}}\n" {
		t.Fatalf("too many bulk names mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMinecraftHasJoinedRouteUsesCurrentActorExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	user := testutil.CreateUser(t, db, "minecraft-route-hasjoined@test.com", "pw", "MinecraftRouteHasJoined", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_route_hasjoined", "MinecraftJoined")
	profileID := profile.ID
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "route-access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggSession(t.Context(), model.Session{ServerID: "route-server", AccessToken: "route-access", CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`{"username":"MinecraftJoined","server_id":"route-server"}`))
	req = req.WithContext(shared.WithActor(req.Context(), clientActorWith("minecraft_session.hasjoined.server")))
	rec := httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("has joined status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	profileBody := body["profile"].(map[string]any)
	if body["joined"] != true || profileBody["id"] != profile.ID || profileBody["name"] != profile.Name {
		t.Fatalf("has joined body mismatch: %#v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`[`))
	req = req.WithContext(shared.WithActor(req.Context(), clientActorWith("minecraft_session.hasjoined.server")))
	rec = httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("has joined invalid json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`{"username":"MinecraftJoined"}`))
	req = req.WithContext(shared.WithActor(req.Context(), clientActorWith("minecraft_session.hasjoined.server")))
	rec = httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"minecraft_session\",\"operation\":\"join\",\"reason\":\"required\"}}\n" {
		t.Fatalf("has joined missing fields mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`{"username":"MinecraftJoined","server_id":"route-server"}`))
	req = req.WithContext(shared.WithActor(req.Context(), clientActorWith()))
	rec = httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("has joined missing permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func clientActorWith(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "client:test-client",
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
		Permissions: bits,
	}
}

func minecraftPublicRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	return req.WithContext(shared.WithActor(req.Context(), permission.GuestActor()))
}
