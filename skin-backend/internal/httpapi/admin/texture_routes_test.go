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
