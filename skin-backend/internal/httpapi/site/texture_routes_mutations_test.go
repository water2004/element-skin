package site_test

import (
	"context"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextureRoutesAddUpdateDeleteAndApplyExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	owner := testutil.CreateUser(t, db, "site-texture-owner@test.com", "Password123", "SiteTextureOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-other@test.com", "Password123", "SiteTextureOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_texture_apply", "SiteTextureApply")
	if err := db.Textures.AddToLibrary(context.Background(), owner.ID, "site_route_public_hash", "skin", "Public Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/site_route_public_hash/wardrobe?texture_type=skin", nil)
	req.SetPathValue("hash", "site_route_public_hash")
	req = withUserActor(req, other.ID)
	rec := httptest.NewRecorder()
	h.AddTexture(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("add texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err := db.Textures.GetInfo(req.Context(), other.ID, "site_route_public_hash", "skin")
	if err != nil || info == nil || info["is_public"] != 2 {
		t.Fatalf("wardrobe add should persist copied texture: info=%#v err=%v", info, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/users/me/textures/site_route_public_hash/skin", strings.NewReader(`{"note":"Mine","model":"slim","is_public":false}`))
	req.SetPathValue("hash", "site_route_public_hash")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, other.ID)
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"note":"Mine"`) || !strings.Contains(rec.Body.String(), `"model":"slim"`) {
		t.Fatalf("update texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/site_route_public_hash/apply", strings.NewReader(`{"profile_id":"`+profile.ID+`","texture_type":"skin"}`))
	req.SetPathValue("hash", "site_route_public_hash")
	req = withUserActor(req, other.ID)
	rec = httptest.NewRecorder()
	h.ApplyTexture(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("apply texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	applied, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || applied == nil || applied.SkinHash == nil || *applied.SkinHash != "site_route_public_hash" {
		t.Fatalf("apply texture should persist profile skin: profile=%#v err=%v", applied, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/users/me/textures/site_route_public_hash/skin", nil)
	req.SetPathValue("hash", "site_route_public_hash")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, other.ID)
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err = db.Textures.GetInfo(req.Context(), other.ID, "site_route_public_hash", "skin")
	if err != nil || info != nil {
		t.Fatalf("delete texture should remove wardrobe row: info=%#v err=%v", info, err)
	}
}

func TestTextureRoutesDeleteMissingWardrobeRowDoesNotClearAppliedProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	owner := testutil.CreateUser(t, db, "site-texture-delete-owner@test.com", "Password123", "SiteTextureDeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-delete-other@test.com", "Password123", "SiteTextureDeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_texture_delete_keeps_profile", "SiteTextureDeleteKeepsProfile")
	if err := db.Textures.AddToLibrary(context.Background(), owner.ID, "site_route_delete_foreign", "skin", "Foreign Texture", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(t.Context(), profile.ID, ptrString("site_route_delete_foreign")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/users/me/textures/site_route_delete_foreign/skin", nil)
	req.SetPathValue("hash", "site_route_delete_foreign")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, other.ID)
	rec := httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"Texture not found\"}\n" {
		t.Fatalf("delete missing wardrobe row should return not found: status=%d body=%q", rec.Code, rec.Body.String())
	}
	applied, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || applied == nil || applied.SkinHash == nil || *applied.SkinHash != "site_route_delete_foreign" {
		t.Fatalf("failed delete of non-wardrobe texture must not clear applied profile hash: profile=%#v err=%v", applied, err)
	}
	info, err := db.Textures.GetInfo(req.Context(), owner.ID, "site_route_delete_foreign", "skin")
	if err != nil || info == nil {
		t.Fatalf("failed delete of non-wardrobe texture must not remove uploader library row: info=%#v err=%v", info, err)
	}
}
