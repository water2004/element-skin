package site_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestProfileRoutesCreateAndListExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-profile@test.com", "Password123", "SiteProfile", false)

	req := httptest.NewRequest(http.MethodPost, "/me/profiles", strings.NewReader(`{"name":"RouteRole","model":"slim"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.CreateProfile(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"RouteRole"`) || !strings.Contains(rec.Body.String(), `"model":"slim"`) {
		t.Fatalf("create profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/me/profiles?limit=1", nil)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
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

	req := httptest.NewRequest(http.MethodPatch, "/me/profiles/"+profile.ID, strings.NewReader(`{"name":"NewRouteRole"}`))
	req.SetPathValue("pid", profile.ID)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UpdateProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("update profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.Name != "NewRouteRole" {
		t.Fatalf("profile rename should persist exactly: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/me/profiles/"+profile.ID+"/skin", nil)
	req.SetPathValue("pid", profile.ID)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ClearProfileSkin(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("clear profile skin response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err = db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash != nil || updated.CapeHash == nil || *updated.CapeHash != cape {
		t.Fatalf("clear skin should clear only skin: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/me/profiles/"+profile.ID+"/cape", nil)
	req.SetPathValue("pid", profile.ID)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ClearProfileCape(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("clear profile cape response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err = db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.CapeHash != nil {
		t.Fatalf("clear cape should clear cape: profile=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/me/profiles/"+profile.ID, nil)
	req.SetPathValue("pid", profile.ID)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
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

	req := httptest.NewRequest(http.MethodPatch, "/me/profiles/"+profile.ID, strings.NewReader(`{"name":"Stolen"}`))
	req.SetPathValue("pid", profile.ID)
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
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
