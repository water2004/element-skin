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

func TestTextureRoutesListAndDetailExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture@test.com", "Password123", "SiteTexture", false)

	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "site_route_hash", "skin", "Site Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v2/users/me/textures?texture_type=skin", nil)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.ListMyTextures(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"site_route_hash"`) || !strings.Contains(rec.Body.String(), `"note":"Site Route Texture"`) {
		t.Fatalf("list textures response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/users/me/textures/site_route_hash/skin", nil)
	req.SetPathValue("hash", "site_route_hash")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.TextureDetail(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"site_route_hash"`) || !strings.Contains(rec.Body.String(), `"type":"skin"`) {
		t.Fatalf("texture detail response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
