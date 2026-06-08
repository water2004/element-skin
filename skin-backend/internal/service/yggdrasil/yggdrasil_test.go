package yggdrasil_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilProfileJSONExactTexturePayload(t *testing.T) {
	skin := "skin_hash"
	cape := "cape_hash"
	ygg := yggdrasil.Yggdrasil{Cfg: config.Config{SiteURL: "https://skin.example/root/"}}

	signed := ygg.ProfileJSON(model.Profile{
		ID: "profile_id", Name: "SlimPlayer", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape,
	}, true)
	if signed["id"] != "profile_id" || signed["name"] != "SlimPlayer" {
		t.Fatalf("unexpected profile envelope: %#v", signed)
	}
	props := signed["properties"].([]map[string]any)
	textureProp := props[0]
	if textureProp["name"] != "textures" || textureProp["signature"] == "" {
		t.Fatalf("signed texture property missing name/signature: %#v", textureProp)
	}
	decoded, err := base64.StdEncoding.DecodeString(textureProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	textures := payload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	if skinPayload["url"] != "https://skin.example/root/static/textures/skin_hash.png" ||
		skinPayload["metadata"].(map[string]any)["model"] != "slim" ||
		textures["CAPE"].(map[string]any)["url"] != "https://skin.example/root/static/textures/cape_hash.png" {
		t.Fatalf("unexpected textures payload: %#v", textures)
	}
	if props[1]["name"] != "uploadableTextures" || props[1]["value"] != "skin,cape" {
		t.Fatalf("missing uploadableTextures property: %#v", props)
	}

	unsigned := ygg.ProfileJSON(model.Profile{ID: "p2", Name: "DefaultPlayer", TextureModel: "default", SkinHash: &skin}, false)
	unsignedProp := unsigned["properties"].([]map[string]any)[0]
	if _, ok := unsignedProp["signature"]; ok {
		t.Fatalf("unsigned profile should not include signature: %#v", unsignedProp)
	}
	decoded, err = base64.StdEncoding.DecodeString(unsignedProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	defaultSkin := payload["textures"].(map[string]any)["SKIN"].(map[string]any)
	if _, ok := defaultSkin["metadata"]; ok {
		t.Fatalf("default model should not include metadata: %#v", defaultSkin)
	}
}

func TestYggdrasilLookupNameReturnsExactStatus(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "lookup-service@test.com", "Password123", "LookupService", false)
	testutil.CreateProfile(t, db, user.ID, "lookup_profile_id", "LookupProfile")
	ygg := yggdrasil.Yggdrasil{DB: db}

	hit, status, err := ygg.LookupName(ctx, "LookupProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || hit["id"] != "lookup_profile_id" || hit["name"] != "LookupProfile" {
		t.Fatalf("unexpected lookup hit status=%d body=%#v", status, hit)
	}
	miss, status, err := ygg.LookupName(ctx, "MissingProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 204 || miss != nil {
		t.Fatalf("unexpected lookup miss status=%d body=%#v", status, miss)
	}
}

func TestYggdrasilAuthRefreshValidateJoinAndMetadata(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	cfg.FallbackDomains = []string{"cdn.example"}
	if err := db.SetSetting(ctx, "site_name", "Exact Ygg"); err != nil {
		t.Fatal(err)
	}
	user := testutil.CreateUser(t, db, "ygg-auth@test.com", "Password123", "YggAuth", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_auth_profile", "YggRole")
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: cfg}

	meta, err := ygg.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	metaBody := meta["meta"].(map[string]any)
	links := metaBody["links"].(map[string]any)
	domains := meta["skinDomains"].([]string)
	if metaBody["serverName"] != "Exact Ygg" || links["homepage"] != "https://skin.example/root/" || links["register"] != "https://skin.example/root/register/" ||
		len(domains) != 2 || domains[0] != "cdn.example" || domains[1] != "skin.example" {
		t.Fatalf("metadata mismatch: %#v", meta)
	}

	auth, err := ygg.Authenticate(ctx, "ygg-auth@test.com", "Password123", "client_token", true)
	if err != nil {
		t.Fatal(err)
	}
	if auth["clientToken"] != "client_token" || auth["accessToken"] == "" {
		t.Fatalf("auth token response mismatch: %#v", auth)
	}
	selected := auth["selectedProfile"].(map[string]any)
	if selected["id"] != profile.ID || selected["name"] != profile.Name {
		t.Fatalf("selected profile mismatch: %#v", selected)
	}
	available := auth["availableProfiles"].([]map[string]any)
	if len(available) != 1 || available[0]["id"] != profile.ID || available[0]["name"] != profile.Name {
		t.Fatalf("available profiles mismatch: %#v", available)
	}
	userPayload := auth["user"].(map[string]any)
	props := userPayload["properties"].([]map[string]any)
	if userPayload["id"] != user.ID || len(props) != 1 || props[0]["name"] != "preferredLanguage" || props[0]["value"] != "zh_CN" {
		t.Fatalf("requestUser payload mismatch: %#v", userPayload)
	}
	access := auth["accessToken"].(string)
	if err := ygg.Validate(ctx, access, "client_token"); err != nil {
		t.Fatalf("fresh token should validate: %v", err)
	}

	if err := ygg.Join(ctx, access, profile.ID, "server_1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	joined, status, err := ygg.HasJoined(ctx, profile.Name, "server_1")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || joined["id"] != profile.ID || joined["name"] != profile.Name {
		t.Fatalf("HasJoined mismatch: status=%d body=%#v", status, joined)
	}
	if miss, status, err := ygg.HasJoined(ctx, "WrongName", "server_1"); err != nil || status != 204 || miss != nil {
		t.Fatalf("wrong name should miss: status=%d body=%#v err=%v", status, miss, err)
	}

	refreshed, err := ygg.Refresh(ctx, access, "client_token", "", true)
	if err != nil {
		t.Fatal(err)
	}
	newAccess := refreshed["accessToken"].(string)
	if newAccess == "" || newAccess == access || refreshed["clientToken"] != "client_token" {
		t.Fatalf("refresh response mismatch: %#v", refreshed)
	}
	if err := ygg.Validate(ctx, access, "client_token"); err == nil {
		t.Fatal("old access token should be invalid after refresh")
	}
	if err := ygg.Validate(ctx, newAccess, "client_token"); err != nil {
		t.Fatalf("new access token should validate: %v", err)
	}

	if err := db.DeleteToken(ctx, newAccess); err != nil {
		t.Fatal(err)
	}
	if err := db.AddToken(ctx, model.Token{AccessToken: "unbound_access", ClientToken: "client_unbound", UserID: user.ID, ProfileID: nil, CreatedAt: database.NowMS()}); err != nil {
		t.Fatal(err)
	}
	bound, err := ygg.Refresh(ctx, "unbound_access", "client_unbound", profile.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	boundSelected := bound["selectedProfile"].(map[string]any)
	if boundSelected["id"] != profile.ID || boundSelected["name"] != profile.Name {
		t.Fatalf("refresh selectedID should bind profile: %#v", bound)
	}

	if _, err := ygg.Authenticate(ctx, profile.Name, "wrong-password", "", false); err == nil || !strings.Contains(err.Error(), "Invalid credentials") {
		t.Fatalf("bad credentials should return ygg error, got %v", err)
	}
}
