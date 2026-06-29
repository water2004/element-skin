package integration_test

import (
	"context"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMicrosoftImportProfileTokenSemantics(t *testing.T) {
	db, h, cache := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, db, "msapi@test.com", "Password123", "MsApiUser", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}

	importToken := "import-token-ok"
	seedMicrosoftState(t, cache, importToken, map[string]any{
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
	})

	resp := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{
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
	if forged, _ := db.Profiles.GetByID(context.Background(), "forged_id"); forged != nil {
		t.Fatal("client-supplied forged profile id should not be persisted")
	}
	if verified, _ := db.Profiles.GetByID(context.Background(), "verified_ms_id"); verified == nil {
		t.Fatal("verified profile should be persisted")
	}

	replay := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": importToken}, cookie)
	if replay.Code != 400 {
		t.Fatalf("import token should be one-time, got %d body=%s", replay.Code, replay.Body.String())
	}

	otherUserToken := "import-token-other-user"
	seedMicrosoftState(t, cache, otherUserToken, map[string]any{
		"user_id": "some-other-user-id",
		"kind":    "import",
		"profile": map[string]any{"id": "x_id", "name": "X"},
	})
	other := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": otherUserToken}, cookie)
	if other.Code != 403 {
		t.Fatalf("other user's token should be 403, got %d body=%s", other.Code, other.Body.String())
	}

	wrongKindToken := "import-token-wrong-kind"
	seedMicrosoftState(t, cache, wrongKindToken, map[string]any{
		"user_id": user.ID,
		"kind":    "profile",
		"profile": map[string]any{"id": "wrong_kind_id", "name": "WrongKind"},
	})
	wrongKind := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": wrongKindToken}, cookie)
	if wrongKind.Code != 400 {
		t.Fatalf("wrong kind token should be 400, got %d body=%s", wrongKind.Code, wrongKind.Body.String())
	}

	missing := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": "does-not-exist"}, cookie)
	if missing.Code != 400 {
		t.Fatalf("missing token should be 400, got %d body=%s", missing.Code, missing.Body.String())
	}

	conflictToken := "import-token-uuid-conflict"
	testutil.CreateProfile(t, db, user.ID, "conflict_ms_id", "ExistingMsProfile")
	seedMicrosoftState(t, cache, conflictToken, map[string]any{
		"user_id": user.ID,
		"kind":    "import",
		"profile": map[string]any{"id": "conflict_ms_id", "name": "ConflictMsPlayer", "skins": []any{}, "capes": []any{}},
	})
	conflict := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": conflictToken}, cookie)
	if conflict.Code != 400 || !strings.Contains(conflict.Body.String(), "UUID") {
		t.Fatalf("uuid conflict should be 400 with UUID detail, got %d body=%s", conflict.Code, conflict.Body.String())
	}

	nameDedupToken := "import-token-name-dedup"
	testutil.CreateProfile(t, db, user.ID, "other_ms_name", "TakenMsName")
	seedMicrosoftState(t, cache, nameDedupToken, map[string]any{
		"user_id": user.ID,
		"kind":    "import",
		"profile": map[string]any{"id": "new_ms_id", "name": "TakenMsName", "skins": []any{}, "capes": []any{}},
	})
	dedup := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": nameDedupToken}, cookie)
	if dedup.Code != 200 {
		t.Fatalf("name dedup import status=%d body=%s", dedup.Code, dedup.Body.String())
	}
	dedupProfile := parseJSON(t, dedup)["profile"].(map[string]any)
	if dedupProfile["id"] != "new_ms_id" || dedupProfile["name"] != "TakenMsName_1" {
		t.Fatalf("name conflict should import with suffix: %#v", dedupProfile)
	}
	if row, _ := db.Profiles.GetByID(context.Background(), "new_ms_id"); row == nil || row.Name != "TakenMsName_1" {
		t.Fatalf("deduped microsoft profile not persisted: %#v", row)
	}
}

func TestMicrosoftAuthURLAndGetProfileTokenSemantics(t *testing.T) {
	userDB, h, cache := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, userDB, "msflow@test.com", "Password123", "MsFlow", false)
	other := testutil.CreateUser(t, userDB, "msflow-other@test.com", "Password123", "MsFlowOther", false)
	token, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, time.Hour)
	otherToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, other.ID, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: token}
	otherCookie := &http.Cookie{Name: "access_token", Value: otherToken}

	authURL := doJSON(t, h, "GET", "/v1/imports/microsoft/auth-url", nil, cookie)
	if authURL.Code != 200 {
		t.Fatalf("auth-url status=%d body=%s", authURL.Code, authURL.Body.String())
	}
	authBody := parseJSON(t, authURL)
	state, _ := authBody["state"].(string)
	if state == "" || !strings.Contains(authBody["auth_url"].(string), "state="+state) {
		t.Fatalf("unexpected auth-url body: %#v", authBody)
	}

	profileToken := "ms-profile-token"
	seedMicrosoftState(t, cache, profileToken, map[string]any{
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
	})
	getProfileOther := doJSON(t, h, "POST", "/v1/imports/microsoft/profile", map[string]any{"ms_token": profileToken}, otherCookie)
	if getProfileOther.Code != 403 {
		t.Fatalf("other user's profile token should be 403, got %d body=%s", getProfileOther.Code, getProfileOther.Body.String())
	}
	// The failed cross-user attempt pops the one-shot token; seed it again for the owner path.
	seedMicrosoftState(t, cache, profileToken, map[string]any{
		"user_id": user.ID,
		"kind":    "profile",
		"profile": map[string]any{
			"has_game": true,
			"profile":  map[string]any{"id": "ms_flow_profile", "name": "MsFlowPlayer", "skins": []any{}, "capes": []any{}},
		},
	})
	getProfile := doJSON(t, h, "POST", "/v1/imports/microsoft/profile", map[string]any{"ms_token": profileToken}, cookie)
	if getProfile.Code != 200 {
		t.Fatalf("get-profile status=%d body=%s", getProfile.Code, getProfile.Body.String())
	}
	getBody := parseJSON(t, getProfile)
	importToken, _ := getBody["import_token"].(string)
	if importToken == "" || getBody["has_game"] != true {
		t.Fatalf("unexpected get-profile body: %#v", getBody)
	}
	replay := doJSON(t, h, "POST", "/v1/imports/microsoft/profile", map[string]any{"ms_token": profileToken}, cookie)
	if replay.Code != 400 {
		t.Fatalf("profile token should be one-shot, got %d body=%s", replay.Code, replay.Body.String())
	}
	importResp := doJSON(t, h, "POST", "/v1/imports/microsoft/profile/import", map[string]any{"ms_token": importToken}, cookie)
	if importResp.Code != 200 {
		t.Fatalf("import issued token status=%d body=%s", importResp.Code, importResp.Body.String())
	}
}

func seedMicrosoftState(t *testing.T, cache redisstore.Store, token string, value map[string]any) {
	t.Helper()
	if err := cache.SetState(context.Background(), token, value, time.Minute); err != nil {
		t.Fatal(err)
	}
}
