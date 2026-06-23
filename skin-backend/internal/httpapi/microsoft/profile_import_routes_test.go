package microsoft_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/microsoft"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestGetProfileConsumesStateAndIssuesImportTokenExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	states := redisstore.NewMemoryStore()
	h := microsoft.New(testutil.TestConfig(), db, settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}, nil, states)
	user := testutil.CreateUser(t, db, "ms-profile@test.com", "Password123", "MSProfile", false)
	if err := microsoft.SeedStateForTest(states, "profile-token", map[string]any{
		"user_id": user.ID,
		"kind":    microsoft.TestStateKindProfile,
		"profile": map[string]any{
			"profile": map[string]any{
				"id":    "ms-profile-id",
				"name":  "MSPlayer",
				"skins": []any{map[string]any{"url": "http://skin", "variant": "slim"}},
			},
			"has_game": true,
		},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/microsoft/get-profile", strings.NewReader(`{"ms_token":"profile-token"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.GetProfile(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"has_game":true`) ||
		!strings.Contains(rec.Body.String(), `"id":"ms-profile-id"`) ||
		!strings.Contains(rec.Body.String(), `"name":"MSPlayer"`) ||
		!strings.Contains(rec.Body.String(), `"import_token":"`) {
		t.Fatalf("get microsoft profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	importToken := jsonStringField(t, rec.Body.String(), "import_token")
	importState := microsoft.PopStateForTest(states, importToken)
	if importState["user_id"] != user.ID || importState["kind"] != microsoft.TestStateKindImport {
		t.Fatalf("get profile should store exact import state: %#v", importState)
	}
	if profile := importState["profile"].(map[string]any); profile["id"] != "ms-profile-id" || profile["name"] != "MSPlayer" {
		t.Fatalf("import state profile mismatch: %#v", importState)
	}

	req = httptest.NewRequest(http.MethodPost, "/microsoft/get-profile", strings.NewReader(`{"ms_token":"profile-token"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.GetProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Invalid or expired token"`) {
		t.Fatalf("profile token should be single-use: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMicrosoftProfileRoutesRejectWrongOwnerExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	states := redisstore.NewMemoryStore()
	h := microsoft.New(testutil.TestConfig(), db, settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}, nil, states)
	owner := testutil.CreateUser(t, db, "ms-owner@test.com", "Password123", "MSOwner", false)
	other := testutil.CreateUser(t, db, "ms-other@test.com", "Password123", "MSOther", false)
	if err := microsoft.SeedStateForTest(states, "owned-token", map[string]any{
		"user_id": owner.ID,
		"kind":    microsoft.TestStateKindProfile,
		"profile": map[string]any{"profile": map[string]any{"id": "id", "name": "Name"}},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/microsoft/get-profile", strings.NewReader(`{"ms_token":"owned-token"}`))
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
	rec := httptest.NewRecorder()
	h.GetProfile(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"Unauthorized"`) {
		t.Fatalf("wrong owner get profile mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestImportProfileRejectsInvalidTokenOwnerAndPayloadExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	states := redisstore.NewMemoryStore()
	h := microsoft.New(testutil.TestConfig(), db, settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}, nil, states)
	owner := testutil.CreateUser(t, db, "ms-import-owner@test.com", "Password123", "MSImportOwner", false)
	other := testutil.CreateUser(t, db, "ms-import-other@test.com", "Password123", "MSImportOther", false)

	req := httptest.NewRequest(http.MethodPost, "/microsoft/import-profile", strings.NewReader(`{"ms_token":"missing"}`))
	req = req.WithContext(shared.WithUser(req.Context(), owner.ID, false))
	rec := httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid import token"`) {
		t.Fatalf("missing import token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := microsoft.SeedStateForTest(states, "wrong-owner", map[string]any{
		"user_id": other.ID,
		"kind":    microsoft.TestStateKindImport,
		"profile": map[string]any{"id": "ms-import-id", "name": "MSImport"},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/microsoft/import-profile", strings.NewReader(`{"ms_token":"wrong-owner"}`))
	req = req.WithContext(shared.WithUser(req.Context(), owner.ID, false))
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"not allowed"`) {
		t.Fatalf("wrong owner import token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := microsoft.SeedStateForTest(states, "bad-payload", map[string]any{"user_id": owner.ID, "kind": microsoft.TestStateKindImport, "profile": "bad"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/microsoft/import-profile", strings.NewReader(`{"ms_token":"bad-payload"}`))
	req = req.WithContext(shared.WithUser(req.Context(), owner.ID, false))
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid import token"`) {
		t.Fatalf("bad import payload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestImportProfileCreatesProfileFromStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	states := redisstore.NewMemoryStore()
	h := microsoft.New(testutil.TestConfig(), db, settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}, nil, states)
	user := testutil.CreateUser(t, db, "ms-import-ok@test.com", "Password123", "MSImportOK", false)
	if err := microsoft.SeedStateForTest(states, "import-ok", map[string]any{
		"user_id": user.ID,
		"kind":    microsoft.TestStateKindImport,
		"profile": map[string]any{
			"id":    "ms_import_ok_profile",
			"name":  "MSImportOK",
			"skins": []any{map[string]any{"url": "http://skin-bytes", "variant": "slim"}},
			"capes": []any{map[string]any{"url": "http://cape-bytes"}},
		},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/microsoft/import-profile", strings.NewReader(`{"ms_token":"import-ok"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) ||
		!strings.Contains(rec.Body.String(), `"id":"ms_import_ok_profile"`) ||
		!strings.Contains(rec.Body.String(), `"name":"MSImportOK"`) {
		t.Fatalf("import profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	profile, err := db.Profiles.GetByID(req.Context(), "ms_import_ok_profile")
	if err != nil || profile == nil || profile.UserID != user.ID || profile.Name != "MSImportOK" ||
		profile.TextureModel != "slim" || profile.SkinHash == nil || profile.CapeHash == nil {
		t.Fatalf("import should persist profile with skin/cape: profile=%#v err=%v", profile, err)
	}
}

func jsonStringField(t *testing.T, body, field string) string {
	t.Helper()
	marker := `"` + field + `":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing field %s in %q", field, body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated field %s in %q", field, body)
	}
	return body[start : start+end]
}
