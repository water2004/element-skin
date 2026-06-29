package site_test

import (
	"context"
	"element-skin/backend/internal/httpapi/site"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileRoutesCreateAndListExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-profile@test.com", "Password123", "SiteProfile", false)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/profiles", strings.NewReader(`{"name":"RouteRole","model":"slim"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.CreateProfile(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"RouteRole"`) || !strings.Contains(rec.Body.String(), `"model":"slim"`) {
		t.Fatalf("create profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/users/me/profiles?limit=1", nil)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ListMyProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"RouteRole"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("list profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestProfileRoutesUpdateClearAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-profile-edit@test.com", "Password123", "SiteProfileEdit", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_profile_edit", "OldRouteRole")
	skin := "route_skin_hash"
	cape := "route_cape_hash"
	if err := db.Profiles.UpdateSkin(context.Background(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(context.Background(), profile.ID, &cape); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/users/me/profiles/"+profile.ID, strings.NewReader(`{"name":"NewRouteRole"}`))
	req.SetPathValue("pid", profile.ID)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("update profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.Name != "NewRouteRole" {
		t.Fatalf("profile rename should persist exactly: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/users/me/profiles/"+profile.ID+"/skin", nil)
	req.SetPathValue("pid", profile.ID)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ClearProfileSkin(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("clear profile skin response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err = db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash != nil || updated.CapeHash == nil || *updated.CapeHash != cape {
		t.Fatalf("clear skin should clear only skin: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/users/me/profiles/"+profile.ID+"/cape", nil)
	req.SetPathValue("pid", profile.ID)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ClearProfileCape(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("clear profile cape response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err = db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.CapeHash != nil {
		t.Fatalf("clear cape should clear cape: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/users/me/profiles/"+profile.ID, nil)
	req.SetPathValue("pid", profile.ID)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.DeleteProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	deleted, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || deleted != nil {
		t.Fatalf("delete profile should remove row: profile=%#v err=%v", deleted, err)
	}
}

func TestProfileRoutesRejectForeignProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	owner := testutil.CreateUser(t, db, "site-profile-owner@test.com", "Password123", "SiteProfileOwner", false)
	other := testutil.CreateUser(t, db, "site-profile-foreign@test.com", "Password123", "SiteProfileForeign", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "site_profile_foreign", "ForeignRouteRole")

	req := httptest.NewRequest(http.MethodPatch, "/v1/users/me/profiles/"+profile.ID, strings.NewReader(`{"name":"Stolen"}`))
	req.SetPathValue("pid", profile.ID)
	req = withUserActor(req, other.ID)
	rec := httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"not allowed"`) {
		t.Fatalf("foreign update should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || unchanged == nil || unchanged.Name != profile.Name {
		t.Fatalf("foreign update must not mutate profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestProfileRoutesRejectInvalidInputsAndConflictsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-profile-errors@test.com", "Password123", "SiteProfileErrors", false)
	existing := testutil.CreateProfile(t, db, user.ID, "site_profile_existing", "ExistingRole")
	target := testutil.CreateProfile(t, db, user.ID, "site_profile_target", "TargetRole")

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/profiles", strings.NewReader(`{`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.CreateProfile(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("create bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/users/me/profiles", strings.NewReader(`{"name":"bad-name!"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.CreateProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "角色名只能包含字母") {
		t.Fatalf("create invalid name mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/users/me/profiles/"+target.ID, strings.NewReader(`{"name":"ExistingRole"}`))
	req.SetPathValue("pid", target.ID)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"角色名已被占用\"}\n" {
		t.Fatalf("rename conflict mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Profiles.GetByID(req.Context(), target.ID)
	if err != nil || unchanged == nil || unchanged.Name != "TargetRole" {
		t.Fatalf("conflicting rename should not mutate profile: profile=%#v err=%v", unchanged, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/users/me/profiles?cursor=not-base64", nil)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ListMyProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/users/me/profiles/missing", nil)
	req.SetPathValue("pid", "missing")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.DeleteProfile(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"profile not found\"}\n" {
		t.Fatalf("delete missing profile mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	stillExisting, err := db.Profiles.GetByID(req.Context(), existing.ID)
	if err != nil || stillExisting == nil || stillExisting.Name != "ExistingRole" {
		t.Fatalf("unrelated profile should remain unchanged: profile=%#v err=%v", stillExisting, err)
	}
}

func TestProfileRoutesRejectForeignTextureClearsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	owner := testutil.CreateUser(t, db, "site-profile-clear-owner@test.com", "Password123", "SiteProfileClearOwner", false)
	other := testutil.CreateUser(t, db, "site-profile-clear-other@test.com", "Password123", "SiteProfileClearOther", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "site_profile_foreign_clear", "ForeignClearRole")
	skin := "foreign_clear_skin"
	cape := "foreign_clear_cape"
	if err := db.Profiles.UpdateSkin(t.Context(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(t.Context(), profile.ID, &cape); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "skin", call: h.ClearProfileSkin},
		{name: "cape", call: h.ClearProfileCape},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/profiles/"+profile.ID+"/"+tc.name, nil)
			req.SetPathValue("pid", profile.ID)
			req = withUserActor(req, other.ID)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"not allowed\"}\n" {
				t.Fatalf("foreign %s clear mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(t.Context(), profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash == nil || *unchanged.SkinHash != skin ||
		unchanged.CapeHash == nil || *unchanged.CapeHash != cape {
		t.Fatalf("foreign clears must preserve both textures: profile=%#v err=%v", unchanged, err)
	}
}
