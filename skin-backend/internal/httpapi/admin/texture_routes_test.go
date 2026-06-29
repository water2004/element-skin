package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

func TestTextureRoutesRejectBadPublicValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture@test.com", "Password123", "AdminTexture", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_route_hash", "skin", "route texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_route_hash", strings.NewReader(`{"type":"skin","is_public":"yes"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_route_hash")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid is_public") {
		t.Fatalf("bad public value should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesRejectInvalidPatchTextureType(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_route_hash", strings.NewReader(`{"type":"elytra","note":"bad"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_route_hash")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid texture_type") {
		t.Fatalf("invalid texture type should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesRejectNoUpdateFieldsAndMissingTextureExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/missing_hash", strings.NewReader(`{"type":"skin"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "missing_hash")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "至少需要一个更新字段") {
		t.Fatalf("empty update should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/missing_hash", strings.NewReader(`{"type":"skin","note":"Nope"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "missing_hash")
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"detail":"Texture not found"`) {
		t.Fatalf("missing texture update should be 404 exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/textures/missing_hash?type=skin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "missing_hash")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "per-user deletion requires user_id") {
		t.Fatalf("non-force delete without user_id mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesListUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-state@test.com", "Password123", "AdminTextureState", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_state_hash", "skin", "Admin Texture State", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/textures?q=Admin%20Texture%20State&type=skin", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Textures(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"admin_state_hash"`) || !strings.Contains(rec.Body.String(), `"name":"Admin Texture State"`) {
		t.Fatalf("texture list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_state_hash", strings.NewReader(`{"type":"skin","note":"Admin Texture Renamed","model":"slim","is_public":false}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_state_hash")
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("texture update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err := db.Textures.GetInfo(req.Context(), user.ID, "admin_state_hash", "skin")
	if err != nil || info == nil || info["note"] != "Admin Texture Renamed" || info["model"] != "slim" || info["is_public"] != 0 {
		t.Fatalf("texture update should persist exactly: info=%#v err=%v", info, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/textures/admin_state_hash?type=skin&user_id="+user.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_state_hash")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"success\":true}\n" {
		t.Fatalf("texture delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err = db.Textures.GetInfo(req.Context(), user.ID, "admin_state_hash", "skin")
	if err != nil || info != nil {
		t.Fatalf("texture should be removed from user library: info=%#v err=%v", info, err)
	}
}

func TestTextureRoutesRejectMalformedAndInvalidModelWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-model@test.com", "Password123", "AdminTextureModel", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_model_hash", "skin", "Original Note", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/textures?cursor=not-base64", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Textures(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("texture list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	for _, cursor := range []string{
		util.EncodeCursor(map[string]any{"last_created_at": 1}),
		util.EncodeCursor(map[string]any{"last_created_at": -1, "last_skin_hash": "hash"}),
	} {
		req = httptest.NewRequest(http.MethodGet, "/v1/admin/textures?cursor="+cursor, nil)
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.Textures(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
			t.Fatalf("texture list malformed cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_model_hash", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_model_hash")
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("texture malformed patch mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_model_hash?type=skin", strings.NewReader(`{"note":"Must Not Persist","model":"wide"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_model_hash")
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid model\"}\n" {
		t.Fatalf("invalid texture model mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	info, err := db.Textures.GetInfo(t.Context(), user.ID, "admin_model_hash", "skin")
	if err != nil || info == nil || info["note"] != "Original Note" || info["model"] != "default" || info["is_public"] != 1 {
		t.Fatalf("rejected patches must preserve texture state: info=%#v err=%v", info, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/textures/missing_hash?type=skin&force=true", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "missing_hash")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"success\":true}\n" {
		t.Fatalf("force delete should be idempotent for a missing texture: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesQueryTypeOverridesBodyTypeExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-type@test.com", "Password123", "AdminTextureType", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_shared_hash", "skin", "Skin Note", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_shared_hash", "cape", "Cape Note", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_shared_hash?type=cape", strings.NewReader(`{"type":"skin","note":"Updated Cape"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_shared_hash")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("query-selected texture update mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	skin, err := db.Textures.GetInfo(t.Context(), user.ID, "admin_shared_hash", "skin")
	if err != nil {
		t.Fatal(err)
	}
	cape, err := db.Textures.GetInfo(t.Context(), user.ID, "admin_shared_hash", "cape")
	if err != nil {
		t.Fatal(err)
	}
	if skin == nil || skin["note"] != "Skin Note" || cape == nil || cape["note"] != "Updated Cape" {
		t.Fatalf("query type must override body type: skin=%#v cape=%#v", skin, cape)
	}
}

func TestAdminTexturePatchRollsBackAllFieldsOnDatabaseFailure(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-rollback@test.com", "Password123", "AdminTextureRollback", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_patch_rollback", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(t.Context(),
		`ALTER TABLE user_textures ADD CONSTRAINT reject_admin_slim_model CHECK (model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/admin_patch_rollback?type=skin",
		strings.NewReader(`{"note":"Changed","model":"slim","is_public":false}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("hash", "admin_patch_rollback")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("admin atomic patch failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err := db.Textures.GetInfo(t.Context(), user.ID, "admin_patch_rollback", "skin")
	if err != nil || info == nil ||
		info["note"] != "Original" ||
		info["model"] != "default" ||
		info["is_public"] != 1 {
		t.Fatalf("failed admin patch changed texture: info=%#v err=%v", info, err)
	}
}

func TestAdminTexturePatchReturnsNotFoundWhenLibraryRowIsDeletedAfterRead(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-delete-race@test.com", "Password123", "AdminTextureDeleteRace", false)
	const hash = "admin_patch_delete_race"
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, hash, "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	var one, lockHolderPID int
	if err := tx.QueryRow(t.Context(), `
		SELECT 1, pg_backend_pid()
		FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
		FOR UPDATE
	`, hash).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/textures/"+hash+"?type=skin", strings.NewReader(`{"note":"Must Not Persist"}`))
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("hash", hash)
		rec := httptest.NewRecorder()
		h.UpdateTexture(rec, req)
		result <- rec
	}()
	waitForBlockedTextureMutation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(t.Context(), `DELETE FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}

	rec := <-result
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"Texture not found\"}\n" {
		t.Fatalf("deleted texture patch should return exact not found: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err := db.Textures.GetInfo(t.Context(), user.ID, hash, "skin")
	if err != nil || info == nil || info["note"] != "Original" {
		t.Fatalf("failed patch changed user texture: info=%#v err=%v", info, err)
	}
}

func waitForBlockedTextureMutation(
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	lockHolderPID int,
	result <-chan *httptest.ResponseRecorder,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case rec := <-result:
			t.Fatalf("texture mutation completed before row-lock release: status=%d body=%q", rec.Code, rec.Body.String())
		default:
		}
		var waiting bool
		if err := db.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("texture mutation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
