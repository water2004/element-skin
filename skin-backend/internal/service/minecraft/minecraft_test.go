package minecraft_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	minecraftsvc "element-skin/backend/internal/service/minecraft"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestMinecraftProfilesReturnExactPublicFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "minecraft-profile@test.com", "pw", "MinecraftProfile", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_profile_exact", "MinecraftExact")
	if err := db.Profiles.UpdateModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	profile.TextureModel = "slim"
	svc := minecraftService(db)

	byName, err := svc.ProfileByName(ctx, permission.GuestActor(), profile.Name)
	if err != nil {
		t.Fatal(err)
	}
	wantByName := map[string]any{
		"id":            profile.ID,
		"name":          profile.Name,
		"owner_user_id": user.ID,
		"texture_model": "slim",
		"public":        true,
	}
	if !reflect.DeepEqual(byName, wantByName) {
		t.Fatalf("profile by name mismatch:\n got=%#v\nwant=%#v", byName, wantByName)
	}

	byID, err := svc.ProfileByID(ctx, permission.GuestActor(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantByID := map[string]any{
		"id":            profile.ID,
		"name":          profile.Name,
		"texture_model": "slim",
		"public":        true,
	}
	if !reflect.DeepEqual(byID, wantByID) {
		t.Fatalf("profile by id mismatch:\n got=%#v\nwant=%#v", byID, wantByID)
	}

	bulk, err := svc.ProfilesByNames(ctx, permission.GuestActor(), []string{profile.Name, "MissingMinecraft"})
	if err != nil {
		t.Fatal(err)
	}
	wantBulk := map[string]any{"items": []map[string]any{wantByID}}
	if !reflect.DeepEqual(bulk, wantBulk) {
		t.Fatalf("profiles by names mismatch:\n got=%#v\nwant=%#v", bulk, wantBulk)
	}
}

func TestMinecraftTexturesPropertyUsesSiteTextureURLAndSignedPayload(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "minecraft-texture@test.com", "pw", "MinecraftTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_texture_profile", "MinecraftTexture")
	skin := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	svc := minecraftService(db)

	body, err := svc.TexturesProperty(ctx, permission.GuestActor(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if body["profile_id"] != profile.ID || body["profile_name"] != profile.Name {
		t.Fatalf("textures property profile fields mismatch: %#v", body)
	}
	property := body["textures_property"].(map[string]any)
	if property["name"] != "textures" || property["value"] == "" || property["signature"] == "" {
		t.Fatalf("textures property fields mismatch: %#v", property)
	}
	decoded, err := base64.StdEncoding.DecodeString(property["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	textures := payload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	wantURL := "https://skin.example/static/textures/" + skin + ".png"
	if skinPayload["url"] != wantURL {
		t.Fatalf("skin URL=%q want %q", skinPayload["url"], wantURL)
	}
}

func TestMinecraftHasJoinedRequiresClientActorAndReturnsExactJoinedStates(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "minecraft-hasjoined@test.com", "pw", "MinecraftHasJoined", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_hasjoined_profile", "MinecraftHasJoined")
	profileID := profile.ID
	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "minecraft-access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggSession(ctx, model.Session{ServerID: "server-hash", AccessToken: "minecraft-access", CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	svc := minecraftServiceWithRedis(db, redis)
	actor := clientActorWith("minecraft_session.hasjoined.server")

	joined, err := svc.HasJoined(ctx, actor, minecraftsvc.HasJoinedRequest{Username: profile.Name, ServerID: "server-hash"})
	if err != nil {
		t.Fatal(err)
	}
	profileBody := joined["profile"].(map[string]any)
	if joined["joined"] != true || profileBody["id"] != profile.ID || profileBody["name"] != profile.Name || profileBody["textures_property"] == nil {
		t.Fatalf("joined response mismatch: %#v", joined)
	}

	miss, err := svc.HasJoined(ctx, actor, minecraftsvc.HasJoinedRequest{Username: profile.Name, ServerID: "missing-server"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(miss, map[string]any{"joined": false, "profile": nil}) {
		t.Fatalf("miss response mismatch: %#v", miss)
	}
	if _, err := svc.HasJoined(ctx, permission.Actor{UserID: user.ID, Permissions: actor.Permissions}, minecraftsvc.HasJoinedRequest{Username: profile.Name, ServerID: "server-hash"}); err == nil {
		t.Fatal("user actor must not call minecraft has-joined app-only endpoint")
	}
}

func TestMinecraftServiceRejectsInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "minecraft-errors@test.com", "pw", "MinecraftErrors", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_errors_profile", "MinecraftErrors")
	svc := minecraftService(db)
	_, err := svc.ProfileByName(ctx, permission.Actor{}, profile.Name)
	assertHTTPError(t, err, 403, "permission.check.denied")
	_, err = svc.TexturesProperty(ctx, permission.Actor{}, profile.ID)
	assertHTTPError(t, err, 403, "permission.check.denied")

	_, err = svc.ProfileByName(ctx, permission.GuestActor(), "missing-name")
	assertHTTPError(t, err, 404, "minecraft_profile.resolve.not_found")
	_, err = svc.ProfileByID(ctx, permission.GuestActor(), "missing-id")
	assertHTTPError(t, err, 404, "minecraft_profile.resolve.not_found")
	_, err = svc.TexturesProperty(ctx, permission.GuestActor(), "missing-id")
	assertHTTPError(t, err, 404, "minecraft_profile.resolve.not_found")

	names := make([]string, 101)
	for i := range names {
		names[i] = "Player"
	}
	_, err = svc.ProfilesByNames(ctx, permission.GuestActor(), names)
	assertHTTPError(t, err, 400, "minecraft_profile.read.exceeded")

	_, err = svc.HasJoined(ctx, clientActorWith("minecraft_session.hasjoined.server"), minecraftsvc.HasJoinedRequest{Username: profile.Name})
	assertHTTPError(t, err, 400, "minecraft_session.join.required")
	_, err = svc.HasJoined(ctx, clientActorWith(), minecraftsvc.HasJoinedRequest{Username: profile.Name, ServerID: "server"})
	assertHTTPError(t, err, 403, "permission.check.denied")
	_, err = svc.HasJoined(ctx, permission.Actor{SessionKind: permission.SessionKindClient, Entrypoint: permission.EntrypointAPI, UserID: user.ID, Permissions: clientActorWith("minecraft_session.hasjoined.server").Permissions}, minecraftsvc.HasJoinedRequest{Username: profile.Name, ServerID: "server"})
	assertHTTPError(t, err, 403, "permission.check.denied")
}

func minecraftService(db *database.DB) minecraftsvc.Service {
	return minecraftServiceWithRedis(db, testutil.NewMemoryRedis())
}

func minecraftServiceWithRedis(db *database.DB, redis redisstore.Store) minecraftsvc.Service {
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	return minecraftsvc.Service{DB: db, Ygg: yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis}}
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

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Error() != detail {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}
