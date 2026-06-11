package site_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	sitesvc "element-skin/backend/internal/service/site"
	texturesvc "element-skin/backend/internal/service/texture"
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

func TestTextureRoutesUploadAndUploadApplyExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture-upload@test.com", "Password123", "SiteTextureUpload", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_upload_apply", "SiteTextureUploadApply")

	req := textureMultipartRequest(t, "/me/textures", map[string]string{
		"texture_type": "skin",
		"note":         "Uploaded Route Texture",
		"is_public":    "true",
		"model":        "slim",
	}, "file", "skin.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"texture_type":"skin"`) {
		t.Fatalf("upload texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	uploadedHash := jsonStringField(t, rec.Body.String(), "hash")
	info, err := db.Textures.GetInfo(req.Context(), user.ID, uploadedHash, "skin")
	if err != nil || info == nil || info["note"] != "Uploaded Route Texture" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("upload texture should persist library row: info=%#v err=%v", info, err)
	}

	req = textureMultipartRequest(t, "/textures/upload", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "skin",
		"model":        "default",
	}, "file", "apply.png", routePNGWithColor(t, 64, 64, color.RGBA{R: 200, G: 80, B: 120, A: 255}))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("upload and apply response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	appliedHash := jsonStringField(t, rec.Body.String(), "hash")
	applied, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || applied == nil || applied.SkinHash == nil || *applied.SkinHash != appliedHash || applied.TextureModel != "default" {
		t.Fatalf("upload and apply should update profile: profile=%#v hash=%q err=%v", applied, appliedHash, err)
	}
}

func textureMultipartRequest(t *testing.T, target string, fields map[string]string, fileField, fileName string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func routePNG(t *testing.T, w, h int) []byte {
	return routePNGWithColor(t, w, h, color.RGBA{R: 80, G: 120, B: 200, A: 255})
}

func routePNGWithColor(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jsonStringField(t *testing.T, body, field string) string {
	t.Helper()
	marker := `"` + field + `":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing field %s in %q", field, body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated field %s in %q", field, body)
	}
	return body[start : start+end]
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

func TestTextureRoutesRejectInvalidInputsWithExactErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture-errors@test.com", "Password123", "SiteTextureErrors", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_errors_profile", "SiteTextureErrorsProfile")

	req := textureMultipartRequest(t, "/me/textures", map[string]string{
		"texture_type": "elytra",
	}, "file", "invalid-type.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid texture_type\"}\n" {
		t.Fatalf("invalid upload texture_type mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/textures/upload", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "elytra",
	}, "file", "invalid-apply-type.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid texture_type\"}\n" {
		t.Fatalf("invalid upload apply type should fail before persisting: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid upload apply should not persist texture rows: count=%d err=%v", count, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/me/textures/missing_hash/apply", strings.NewReader(`{"profile_id":"`+profile.ID+`","texture_type":"skin"}`))
	req.SetPathValue("hash", "missing_hash")
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ApplyTexture(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"Texture not found in your library\"}\n" {
		t.Fatalf("apply missing texture mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/me/textures/missing_hash/skin", strings.NewReader(`{`))
	req.SetPathValue("hash", "missing_hash")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.UpdateTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("bad update json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/me/textures/missing_hash/skin", nil)
	req.SetPathValue("hash", "missing_hash")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.TextureDetail(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"Texture not found\"}\n" {
		t.Fatalf("missing texture detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesRejectMalformedUploadsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture-upload-errors@test.com", "Password123", "SiteTextureUploadErrors", false)

	req := httptest.NewRequest(http.MethodPost, "/me/textures", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("malformed upload multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/me/textures", map[string]string{"texture_type": "skin"}, "not_file", "skin.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"file is required"`) {
		t.Fatalf("missing upload file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = textureMultipartRequest(t, "/textures/upload", map[string]string{"texture_type": "skin"}, "file", "skin.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"uuid and texture_type are required\"}\n" {
		t.Fatalf("upload apply missing uuid mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if count, err := db.Textures.CountForUser(req.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid upload attempts should not persist texture rows: count=%d err=%v", count, err)
	}
}

func TestTextureRoutesUploadApplyFailureKeepsUploadedLibraryRow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture-apply-owner@test.com", "Password123", "SiteTextureApplyOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-apply-foreign@test.com", "Password123", "SiteTextureApplyForeign", false)
	foreignProfile := testutil.CreateProfile(t, db, other.ID, "site_texture_foreign_apply", "SiteTextureForeignApply")

	req := textureMultipartRequest(t, "/textures/upload", map[string]string{
		"uuid":         foreignProfile.ID,
		"texture_type": "skin",
		"model":        "slim",
	}, "file", "skin.png", routePNGWithColor(t, 64, 64, color.RGBA{R: 20, G: 180, B: 120, A: 255}))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"Profile not yours\"}\n" {
		t.Fatalf("upload apply foreign profile mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	page, err := db.Textures.ListForUser(req.Context(), user.ID, "skin", 10, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["model"] != "slim" {
		t.Fatalf("upload is persisted before apply failure, so library row should remain: %#v", page)
	}
	foreign, err := db.Profiles.GetByID(req.Context(), foreignProfile.ID)
	if err != nil || foreign == nil || foreign.SkinHash != nil {
		t.Fatalf("failed foreign apply must not mutate foreign profile: profile=%#v err=%v", foreign, err)
	}
}

func TestTextureUploadRemovesNewFileWhenDatabaseInsertFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-texture-db-fail@test.com", "Password123", "SiteTextureDBFail", false)
	if _, err := db.Pool.Exec(t.Context(), `ALTER TABLE user_textures ADD CONSTRAINT reject_test_upload CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	req := textureMultipartRequest(t, "/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "skin.png", routePNG(t, 64, 64))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("database upload failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(t.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("failed database insert must leave no user texture row: count=%d err=%v", count, err)
	}
	entries, err := os.ReadDir(cfg.TexturesDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed database insert must remove the newly-created texture file: entries=%#v err=%v", entries, err)
	}
}

func TestTextureUploadKeepsNewFileWhenAnotherTextureTypeReferencesHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	owner := testutil.CreateUser(t, db, "site-texture-existing-owner@test.com", "Password123", "SiteTextureExistingOwner", false)
	uploader := testutil.CreateUser(t, db, "site-texture-existing-uploader@test.com", "Password123", "SiteTextureExistingUploader", false)
	data := routePNG(t, 64, 64)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	hash := texturesvc.TexturePixelHash(img)
	if err := db.Textures.AddToLibrary(t.Context(), owner.ID, hash, "cape", "Existing Cape Reference", false, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(t.Context(), `ALTER TABLE user_textures ADD CONSTRAINT reject_test_duplicate_upload CHECK (FALSE) NOT VALID`); err != nil {
		t.Fatal(err)
	}

	req := textureMultipartRequest(t, "/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "skin.png", data)
	req = req.WithContext(shared.WithUser(req.Context(), uploader.ID, false))
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("duplicate reference upload failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.TexturesDir, hash+".png")); err != nil {
		t.Fatalf("failed upload must keep a file referenced by another library row: %v", err)
	}
	if info, err := db.Textures.GetInfo(t.Context(), owner.ID, hash, "cape"); err != nil || info == nil {
		t.Fatalf("existing cross-type texture reference must remain: info=%#v err=%v", info, err)
	}
	if count, err := db.Textures.CountForUser(t.Context(), uploader.ID); err != nil || count != 0 {
		t.Fatalf("failed uploader must gain no texture row: count=%d err=%v", count, err)
	}
}

func TestTextureRoutesDeleteMissingWardrobeRowDoesNotClearAppliedProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	owner := testutil.CreateUser(t, db, "site-texture-delete-owner@test.com", "Password123", "SiteTextureDeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-delete-other@test.com", "Password123", "SiteTextureDeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_texture_delete_keeps_profile", "SiteTextureDeleteKeepsProfile")
	if err := db.Textures.AddToLibrary(context.Background(), owner.ID, "site_route_delete_foreign", "skin", "Foreign Texture", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(t.Context(), profile.ID, ptrString("site_route_delete_foreign")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/me/textures/site_route_delete_foreign/skin", nil)
	req.SetPathValue("hash", "site_route_delete_foreign")
	req.SetPathValue("texture_type", "skin")
	req = req.WithContext(shared.WithUser(req.Context(), other.ID, false))
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

func ptrString(s string) *string {
	return &s
}
