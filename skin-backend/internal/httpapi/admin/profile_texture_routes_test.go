package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestProfileTextureRoutesUpdateProfileAndRejectBadTexturePublicValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-profile@test.com", "Password123", "AdminProfile", false)
	profile := testutil.CreateProfile(t, db, user.ID, "admin_route_profile", "AdminRouteProfile")

	req := httptest.NewRequest(http.MethodPatch, "/admin/profiles/"+profile.ID, strings.NewReader(`{"name":"RenamedAdmin"}`))
	req.SetPathValue("profile_id", profile.ID)
	rec := httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("profile update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.GetProfileByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.Name != "RenamedAdmin" {
		t.Fatalf("profile name should persist exactly: profile=%#v err=%v", updated, err)
	}

	if err := db.AddTextureToLibrary(req.Context(), user.ID, "admin_route_hash", "skin", "route texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPatch, "/admin/textures/admin_route_hash", strings.NewReader(`{"is_public":"yes"}`))
	req.SetPathValue("hash", "admin_route_hash")
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid is_public") {
		t.Fatalf("bad public value should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
