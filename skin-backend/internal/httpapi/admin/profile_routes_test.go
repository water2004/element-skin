package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestProfileRoutesUpdateProfilePersistsName(t *testing.T) {
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
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.Name != "RenamedAdmin" {
		t.Fatalf("profile name should persist exactly: profile=%#v err=%v", updated, err)
	}
}
