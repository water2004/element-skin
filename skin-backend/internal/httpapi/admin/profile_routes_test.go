package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestProfileRoutesUpdateProfilePersistsName(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-profile@test.com", "Password123", "AdminProfile", false)
	profile := testutil.CreateProfile(t, db, user.ID, "admin_route_profile", "AdminRouteProfile")

	req := httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+profile.ID, strings.NewReader(`{"name":"RenamedAdmin"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", profile.ID)
	rec := httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
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
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, skinHash, "skin", "admin skin", false, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, capeHash, "cape", "admin cape", false, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/profiles?q=AdminProfileTexture", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"AdminProfileTexture"`) {
		t.Fatalf("profile list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+profile.ID+"/skin", strings.NewReader(`{"hash":"`+skinHash+`"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileSkin(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("profile skin update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+profile.ID+"/cape", strings.NewReader(`{"hash":"`+capeHash+`"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileCape(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("profile cape update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skinHash || updated.CapeHash == nil || *updated.CapeHash != capeHash {
		t.Fatalf("profile texture patch should persist exactly: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/profiles/"+profile.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.DeleteProfile(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("profile delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if deleted, err := db.Profiles.GetByID(req.Context(), profile.ID); err != nil || deleted != nil {
		t.Fatalf("profile should be deleted: profile=%#v err=%v", deleted, err)
	}
}

func TestProfileRoutesRejectInvalidInputsAndConflictsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	user := testutil.CreateUser(t, db, "admin-profile-errors@test.com", "Password123", "AdminProfileErrors", false)
	existing := testutil.CreateProfile(t, db, user.ID, "admin_profile_existing", "AdminExisting")
	target := testutil.CreateProfile(t, db, user.ID, "admin_profile_target", "AdminTarget")
	skinHash := "admin_error_skin"
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, skinHash, "skin", "existing skin", false, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(context.Background(), target.ID, &skinHash); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/profiles?cursor=not-base64", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"pagination_cursor\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	incompleteCursor := util.EncodeCursor(map[string]any{"unexpected": "value"})
	req = httptest.NewRequest(http.MethodGet, "/v2/admin/profiles?cursor="+incompleteCursor, nil)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"pagination_cursor\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile list incomplete cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID, strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile update bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID, strings.NewReader(`{"name":"bad-name!"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"profile_name\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile update invalid name mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID, strings.NewReader(`{"name":"AdminExisting"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusConflict || rec.Body.String() != "{\"error\":{\"object\":\"profile_name\",\"operation\":\"reserve\",\"reason\":\"conflict\"}}\n" {
		t.Fatalf("profile update name conflict mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Profiles.GetByID(req.Context(), target.ID)
	if err != nil || unchanged == nil || unchanged.Name != "AdminTarget" {
		t.Fatalf("conflicting admin rename should not mutate profile: profile=%#v err=%v", unchanged, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/missing", strings.NewReader(`{"name":"ValidName"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", "missing")
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("profile update missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID+"/skin", strings.NewReader(`{"hash":"missing_texture_hash"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileSkin(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"texture\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("profile skin missing texture mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchangedTexture, err := db.Profiles.GetByID(req.Context(), target.ID)
	if err != nil || unchangedTexture == nil || unchangedTexture.SkinHash == nil || *unchangedTexture.SkinHash != skinHash {
		t.Fatalf("missing texture should not mutate profile: profile=%#v err=%v", unchangedTexture, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID+"/skin", strings.NewReader(`{"hash":null}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileSkin(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("profile skin clear response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cleared, err := db.Profiles.GetByID(req.Context(), target.ID)
	if err != nil || cleared == nil || cleared.SkinHash != nil {
		t.Fatalf("null hash should clear profile skin: profile=%#v err=%v", cleared, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/missing/skin", strings.NewReader(`{"hash":"anything"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", "missing")
	rec = httptest.NewRecorder()
	h.UpdateProfileSkin(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("profile skin update missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if got, err := db.Profiles.GetByID(req.Context(), existing.ID); err != nil || got == nil || got.Name != "AdminExisting" {
		t.Fatalf("existing profile should remain unchanged: profile=%#v err=%v", got, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/profiles", nil)
	rec = httptest.NewRecorder()
	h.Profiles(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("profile list permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID, strings.NewReader(`{"name":"Denied"}`))
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("profile update permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/profiles/"+target.ID, nil)
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.DeleteProfile(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("profile delete permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/profiles/"+target.ID+"/cape", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("profile_id", target.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfileCape(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile cape bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
