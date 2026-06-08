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

func TestProfileRoutesListDeleteAndTexturePatchExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-profile-texture@test.com", "Password123", "AdminProfileTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "admin_profile_texture", "AdminProfileTexture")
	skinHash := "admin_profile_skin_hash"
	capeHash := "admin_profile_cape_hash"

	req := httptest.NewRequest(http.MethodGet, "/admin/profiles?q=AdminProfileTexture", nil)
	rec := httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"AdminProfileTexture"`) {
		t.Fatalf("profile list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/admin/profiles/"+profile.ID+"/skin", strings.NewReader(`{"hash":"`+skinHash+`"}`))
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileSkin(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("profile skin update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPatch, "/admin/profiles/"+profile.ID+"/cape", strings.NewReader(`{"hash":"`+capeHash+`"}`))
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileCape(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("profile cape update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skinHash || updated.CapeHash == nil || *updated.CapeHash != capeHash {
		t.Fatalf("profile texture patch should persist exactly: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/profiles/"+profile.ID, nil)
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.DeleteProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("profile delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if deleted, err := db.Profiles.GetByID(req.Context(), profile.ID); err != nil || deleted != nil {
		t.Fatalf("profile should be deleted: profile=%#v err=%v", deleted, err)
	}
}
