package site_test

import (
	"bytes"
	"element-skin/backend/internal/httpapi/site"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextureRoutesUploadAndUploadApplyExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-upload@test.com", "Password123", "SiteTextureUpload", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_upload_apply", "SiteTextureUploadApply")

	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "skin",
		"note":         "Uploaded Route Texture",
		"is_public":    "true",
		"model":        "slim",
	}, "file", "skin.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"texture_type":"skin"`) {
		t.Fatalf("upload texture response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	uploadedHash := jsonStringField(t, rec.Body.String(), "hash")
	info, err := db.Textures.GetInfo(req.Context(), user.ID, uploadedHash, "skin")
	if err != nil || info == nil || info["note"] != "Uploaded Route Texture" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("upload texture should persist library row: info=%#v err=%v", info, err)
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"note": "Default Skin Type",
	}, "file", "default-skin.png", routePNGWithColor(t, 64, 64, color.RGBA{R: 90, G: 180, B: 40, A: 255}))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadMyTexture(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"texture_type":"skin"`) {
		t.Fatalf("upload default texture_type response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	defaultHash := jsonStringField(t, rec.Body.String(), "hash")
	defaultInfo, err := db.Textures.GetInfo(req.Context(), user.ID, defaultHash, "skin")
	if err != nil || defaultInfo == nil || defaultInfo["note"] != "Default Skin Type" || defaultInfo["is_public"] != 0 {
		t.Fatalf("empty texture_type should persist as private skin: info=%#v err=%v", defaultInfo, err)
	}

	applyData := routePNGWithColor(t, 64, 64, color.RGBA{R: 200, G: 80, B: 120, A: 255})
	storage, err := texturesvc.NewTextureStorage(cfg.TexturesDir)
	if err != nil {
		t.Fatal(err)
	}
	preexistingHash, _, err := storage.ProcessAndSaveTracked(applyData, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(req.Context(), user.ID, preexistingHash, "skin", "Existing model", false, "default"); err != nil {
		t.Fatal(err)
	}

	req = textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "skin",
		"model":        "slim",
	}, "file", "apply.png", applyData)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.UploadAndApplyTexture(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"texture_type":"skin"`) {
		t.Fatalf("upload and apply response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	appliedHash := jsonStringField(t, rec.Body.String(), "hash")
	applied, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if appliedHash != preexistingHash || err != nil || applied == nil || applied.SkinHash == nil || *applied.SkinHash != appliedHash || applied.TextureModel != "slim" {
		t.Fatalf("upload and apply should update profile: profile=%#v hash=%q err=%v", applied, appliedHash, err)
	}
}

func TestTextureRoutesUploadApplyFailureKeepsUploadedLibraryRow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-apply-owner@test.com", "Password123", "SiteTextureApplyOwner", false)
	other := testutil.CreateUser(t, db, "site-texture-apply-foreign@test.com", "Password123", "SiteTextureApplyForeign", false)
	foreignProfile := testutil.CreateProfile(t, db, other.ID, "site_texture_foreign_apply", "SiteTextureForeignApply")

	req := textureMultipartRequest(t, "/v2/users/me/textures/upload-and-apply", map[string]string{
		"uuid":         foreignProfile.ID,
		"texture_type": "skin",
		"model":        "slim",
	}, "file", "skin.png", routePNGWithColor(t, 64, 64, color.RGBA{R: 20, G: 180, B: 120, A: 255}))
	req = withUserActor(req, user.ID)
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
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-texture-db-fail@test.com", "Password123", "SiteTextureDBFail", false)
	if _, err := db.Pool.Exec(t.Context(), `ALTER TABLE user_textures ADD CONSTRAINT reject_test_upload CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "skin.png", routePNG(t, 64, 64))
	req = withUserActor(req, user.ID)
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
	h := site.New(cfg, db, nil)
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

	req := textureMultipartRequest(t, "/v2/users/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "skin.png", data)
	req = withUserActor(req, uploader.ID)
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
