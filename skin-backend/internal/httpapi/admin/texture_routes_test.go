package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestTextureRoutesRejectBadPublicValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture@test.com", "Password123", "AdminTexture", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_route_hash", "skin", "route texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/admin/textures/admin_route_hash", strings.NewReader(`{"type":"skin","is_public":"yes"}`))
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
	req := httptest.NewRequest(http.MethodPatch, "/admin/textures/admin_route_hash", strings.NewReader(`{"type":"elytra","note":"bad"}`))
	req.SetPathValue("hash", "admin_route_hash")
	rec := httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid texture_type") {
		t.Fatalf("invalid texture type should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesListUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-texture-state@test.com", "Password123", "AdminTextureState", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "admin_state_hash", "skin", "Admin Texture State", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/textures?q=Admin%20Texture%20State&type=skin", nil)
	rec := httptest.NewRecorder()
	h.Textures(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"admin_state_hash"`) || !strings.Contains(rec.Body.String(), `"name":"Admin Texture State"`) {
		t.Fatalf("texture list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/admin/textures/admin_state_hash", strings.NewReader(`{"type":"skin","note":"Admin Texture Renamed","model":"slim","is_public":false}`))
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

	req = httptest.NewRequest(http.MethodDelete, "/admin/textures/admin_state_hash?type=skin&user_id="+user.ID, nil)
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
