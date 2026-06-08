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

func TestTextureRoutesListAndDetailExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture@test.com", "Password123", "SiteTexture", false)

	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "site_route_hash", "skin", "Site Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/me/textures?texture_type=skin", nil)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.ListMyTextures(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"site_route_hash"`) || !strings.Contains(rec.Body.String(), `"note":"Site Route Texture"`) {
		t.Fatalf("list textures response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/me/textures/site_route_hash/skin", nil)
	req.SetPathValue("hash", "site_route_hash")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.TextureDetail(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"site_route_hash"`) || !strings.Contains(rec.Body.String(), `"type":"skin"`) {
		t.Fatalf("texture detail response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesAddUpdateDeleteAndApplyExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	owner := testutil.CreateUser(t, db, "site-texture-owner@test.com", "Password123", "SiteTextureOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-other@test.com", "Password123", "SiteTextureOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_texture_apply", "SiteTextureApply")
	if err := db.Textures.AddToLibrary(context.Background(), owner.ID, "site_route_public_hash", "skin", "Public Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/me/textures/site_route_public_hash/add?texture_type=skin", nil)
	req.SetPathValue("hash", "site_route_public_hash")
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
	rec := httptest.NewRecorder()
	h.AddTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("add texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err := db.Textures.GetInfo(req.Context(), other.ID, "site_route_public_hash", "skin")
	if err != nil || info == nil || info["is_public"] != 2 {
		t.Fatalf("wardrobe add should persist copied texture: info=%#v err=%v", info, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/me/textures/site_route_public_hash/skin", strings.NewReader(`{"note":"Mine","model":"slim","is_public":false}`))
	req.SetPathValue("hash", "site_route_public_hash")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"note":"Mine"`) || !strings.Contains(rec.Body.String(), `"model":"slim"`) {
		t.Fatalf("update texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/me/textures/site_route_public_hash/apply", strings.NewReader(`{"profile_id":"`+profile.ID+`","texture_type":"skin"}`))
	req.SetPathValue("hash", "site_route_public_hash")
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
	rec = httptest.NewRecorder()
	h.ApplyTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("apply texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	applied, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || applied == nil || applied.SkinHash == nil || *applied.SkinHash != "site_route_public_hash" {
		t.Fatalf("apply texture should persist profile skin: profile=%#v err=%v", applied, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/me/textures/site_route_public_hash/skin", nil)
	req.SetPathValue("hash", "site_route_public_hash")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	info, err = db.Textures.GetInfo(req.Context(), other.ID, "site_route_public_hash", "skin")
	if err != nil || info != nil {
		t.Fatalf("delete texture should remove wardrobe row: info=%#v err=%v", info, err)
	}
}
