package site_test

import (
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/testutil"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextureRoutesRejectMissingFineGrainedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-permissions@test.com", "Password123", "SiteTexturePermissions", false)

	cases := []struct {
		name        string
		permission  string
		makeRequest func() *http.Request
		call        func(http.ResponseWriter, *http.Request)
	}{
		{
			name:       "list requires read",
			permission: "texture.read.owned",
			makeRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/v2/users/me/textures", nil)
			},
			call: h.ListMyTextures,
		},
		{
			name:       "upload requires create",
			permission: "texture.create.owned",
			makeRequest: func() *http.Request {
				return textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{"texture_type": "skin"}, "file", "skin.png", routePNG(t, 64, 64))
			},
			call: h.UploadMyTexture,
		},
		{
			name:       "upload apply requires create",
			permission: "texture.create.owned",
			makeRequest: func() *http.Request {
				return textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{"uuid": "profile-id", "texture_type": "skin"}, "file", "skin.png", routePNG(t, 64, 64))
			},
			call: h.UploadAndApplyTexture,
		},
		{
			name:       "upload apply requires apply",
			permission: "texture.apply.owned",
			makeRequest: func() *http.Request {
				return textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{"uuid": "profile-id", "texture_type": "skin"}, "file", "skin.png", routePNG(t, 64, 64))
			},
			call: h.UploadAndApplyTexture,
		},
		{
			name:       "detail requires read",
			permission: "texture.read.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v2/users/me/textures/hash/skin", nil)
				req.SetPathValue("hash", "hash")
				req.SetPathValue("texture_type", "skin")
				return req
			},
			call: h.TextureDetail,
		},
		{
			name:       "update note requires metadata",
			permission: "texture.update_metadata.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPatch, "/v2/users/me/textures/hash/skin", strings.NewReader(`{"note":"blocked"}`))
				req.SetPathValue("hash", "hash")
				req.SetPathValue("texture_type", "skin")
				return req
			},
			call: h.UpdateTexture,
		},
		{
			name:       "update model requires metadata",
			permission: "texture.update_metadata.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPatch, "/v2/users/me/textures/hash/skin", strings.NewReader(`{"model":"slim"}`))
				req.SetPathValue("hash", "hash")
				req.SetPathValue("texture_type", "skin")
				return req
			},
			call: h.UpdateTexture,
		},
		{
			name:       "update visibility requires visibility",
			permission: "texture.update_visibility.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPatch, "/v2/users/me/textures/hash/skin", strings.NewReader(`{"is_public":true}`))
				req.SetPathValue("hash", "hash")
				req.SetPathValue("texture_type", "skin")
				return req
			},
			call: h.UpdateTexture,
		},
		{
			name:       "delete requires delete",
			permission: "texture.delete.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodDelete, "/v2/users/me/textures/hash/skin", nil)
				req.SetPathValue("hash", "hash")
				req.SetPathValue("texture_type", "skin")
				return req
			},
			call: h.DeleteTexture,
		},
		{
			name:       "wardrobe add requires wardrobe entry add",
			permission: "wardrobe_entry.add.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/hash/wardrobe?texture_type=skin", nil)
				req.SetPathValue("hash", "hash")
				return req
			},
			call: h.AddTexture,
		},
		{
			name:       "apply requires apply",
			permission: "texture.apply.owned",
			makeRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/hash/apply", strings.NewReader(`{"profile_id":"profile","texture_type":"skin"}`))
				req.SetPathValue("hash", "hash")
				return req
			},
			call: h.ApplyTexture,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withUserActorWithoutPermission(tc.makeRequest(), user.ID, tc.permission)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("permission denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTextureRoutesIsPublicWithoutPermissionReturns403(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-no-vis@test.com", "Password123", "SiteTextureNoVis", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_no_vis", "SiteTextureNoVis")

	// UploadMyTexture with is_public=true but without texture.update_visibility.owned
	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "skin",
		"is_public":    "true",
	}, "file", "skin.png", routePNG(t, 64, 64))
	req = withUserActorWithoutPermission(req, user.ID, "texture.update_visibility.owned")
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("is_public without update_visibility should return 403: status=%d body=%q", rec.Code, rec.Body.String())
	}

	// UploadAndApplyTexture with is_public=true but without texture.update_visibility.owned
	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "skin",
		"is_public":    "true",
	}, "file", "apply.png", routePNGWithColor(t, 64, 64, color.RGBA{R: 200, G: 80, B: 120, A: 255}))
	req = withUserActorWithoutPermission(req, user.ID, "texture.update_visibility.owned")
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("upload apply is_public without update_visibility should return 403: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("rejected is_public upload must not persist texture rows: count=%d err=%v", count, err)
	}
}
