package site_test

import (
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextureRoutesRejectInvalidInputsWithExactErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-errors@test.com", "Password123", "SiteTextureErrors", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_errors_profile", "SiteTextureErrorsProfile")

	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "elytra",
	}, "file", "invalid-type.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_type\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid upload texture_type mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "elytra",
	}, "file", "invalid-apply-type.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_type\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid upload apply type should fail before persisting: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid upload apply should not persist texture rows: count=%d err=%v", count, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/missing_hash/wardrobe?texture_type=skin", nil)
	req.SetPathValue("hash", "missing_hash")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.AddTexture(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"texture\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("add missing texture mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("failed wardrobe add should not persist texture rows: count=%d err=%v", count, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/missing_hash/apply", strings.NewReader(`{"profile_id":"`+profile.ID+`","texture_type":"skin"}`))
	req.SetPathValue("hash", "missing_hash")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ApplyTexture(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"texture\",\"operation\":\"authorize\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("apply missing texture mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/users/me/textures/missing_hash/skin", strings.NewReader(`{`))
	req.SetPathValue("hash", "missing_hash")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("bad update json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/users/me/textures/missing_hash/skin", nil)
	req.SetPathValue("hash", "missing_hash")
	req.SetPathValue("texture_type", "skin")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.TextureDetail(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"texture\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing texture detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesRejectMalformedUploadsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-upload-errors@test.com", "Password123", "SiteTextureUploadErrors", false)

	req := httptest.NewRequest(http.MethodPost, "/v2/users/me/textures", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("malformed upload multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{"texture_type": "skin"}, "not_file", "skin.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"upload_file\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
		t.Fatalf("missing upload file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/upload-and-apply", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("malformed upload apply multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{"texture_type": "skin"}, "file", "skin.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture\",\"operation\":\"upload\",\"reason\":\"required\"}}\n" {
		t.Fatalf("upload apply missing uuid mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{"uuid": "profile-id", "texture_type": "skin"}, "not_file", "skin.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"upload_file\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
		t.Fatalf("upload apply missing file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{"uuid": "profile-id", "texture_type": "skin"}, "file", "bad.png", []byte("not a valid png image"))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_format\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n" {
		t.Fatalf("upload apply invalid image mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/textures/hash/apply", strings.NewReader(`{`))
	req.SetPathValue("hash", "hash")
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("apply bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid upload attempts should not persist texture rows: count=%d err=%v", count, err)
	}
}

func TestTextureRoutesInvalidImageReturns400(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-invalid-img@test.com", "Password123", "SiteTextureInvalidImg", false)

	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "bad.png", []byte("not a valid png image"))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_format\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n" {
		t.Fatalf("invalid image should return 400: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid image upload must not persist texture rows: count=%d err=%v", count, err)
	}
}
