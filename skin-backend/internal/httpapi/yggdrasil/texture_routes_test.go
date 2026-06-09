package yggdrasil_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestTextureRoutesRequireBearerAndDeleteClearsProfileSkinExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-texture@test.com", "Password123", "YggTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_texture_profile", "YggTextureProfile")

	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", strings.NewReader(""))
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Bearer token required") {
		t.Fatalf("upload without bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	profileID := profile.ID
	if err := redis.SetYggToken(context.Background(), model.Token{AccessToken: "delete_texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: time.Now().UnixMilli()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if token, err := db.Tokens.Get(context.Background(), "delete_texture_token"); err != nil || token != nil {
		t.Fatalf("texture route seed token must be redis-only: %#v err=%v", token, err)
	}
	skin := "skin_before_delete"
	if err := db.Profiles.UpdateSkin(req.Context(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/skin", nil)
	req.Header.Set("Authorization", "Bearer delete_texture_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete texture should return 204 exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash != nil {
		t.Fatalf("skin hash should be cleared exactly: profile=%#v err=%v", updated, err)
	}
}

func TestTextureRouteUploadUsesRedisYggToken(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload@test.com", "Password123", "YggUpload", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_profile", "YggUploadProfile")
	token := model.Token{AccessToken: "upload_texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "slim"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "skin.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer upload_texture_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("upload texture should be exact 204 empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(context.Background(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || updated.TextureModel != "slim" {
		t.Fatalf("upload should apply skin/model: profile=%#v err=%v", updated, err)
	}
	if dbToken, err := db.Tokens.Get(context.Background(), token.AccessToken); err != nil || dbToken != nil {
		t.Fatalf("upload token must remain redis-only: %#v err=%v", dbToken, err)
	}
}

func TestTextureRoutesRejectProfileMismatchAndInvalidTypeExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-texture-rules@test.com", "Password123", "YggTextureRules", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_texture_rules_profile", "YggTextureRulesProfile")
	other := testutil.CreateProfile(t, db, user.ID, "ygg_texture_rules_other", "YggTextureRulesOther")
	token := model.Token{AccessToken: "texture_rules_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+other.ID+"/skin", nil)
	req.Header.Set("Authorization", "Bearer texture_rules_token")
	req.SetPathValue("uuid", other.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"detail":"Invalid token"`) {
		t.Fatalf("profile mismatch should be generic 401 invalid token: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/elytra", nil)
	req.Header.Set("Authorization", "Bearer texture_rules_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "elytra")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Invalid texture_type"`) {
		t.Fatalf("invalid texture type mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureUploadRejectsInvalidTypeBeforeParsingMultipartOrWritingRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload-invalid-type@test.com", "Password123", "YggUploadInvalidType", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_invalid_type_profile", "YggUploadInvalidTypeProfile")
	token := model.Token{AccessToken: "invalid_type_upload_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/elytra", strings.NewReader("not multipart and must not be parsed"))
	req.Header.Set("Authorization", "Bearer invalid_type_upload_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "elytra")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid texture_type\"}\n" {
		t.Fatalf("invalid upload texture type mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(context.Background(), user.ID); err != nil || count != 0 {
		t.Fatalf("invalid upload type should not write user texture rows: count=%d err=%v", count, err)
	}
	updated, err := db.Profiles.GetByID(context.Background(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash != nil || updated.CapeHash != nil {
		t.Fatalf("invalid upload type should not mutate profile textures: profile=%#v err=%v", updated, err)
	}
}

func TestTextureUploadRejectsUnboundTokenAndBadMultipartExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload-rules@test.com", "Password123", "YggUploadRules", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_rules_profile", "YggUploadRulesProfile")
	if err := redis.SetYggToken(context.Background(), model.Token{AccessToken: "unbound_upload_token", ClientToken: "client", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer unbound_upload_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"detail":"Invalid token"`) {
		t.Fatalf("unbound upload token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := redis.SetYggToken(context.Background(), model.Token{AccessToken: "bound_upload_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", strings.NewReader("not multipart"))
	req.Header.Set("Authorization", "Bearer bound_upload_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec = httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid multipart form"`) {
		t.Fatalf("bad multipart upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureUploadRejectsMissingFileAndNonPNGExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload-file@test.com", "Password123", "YggUploadFile", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_file_profile", "YggUploadFileProfile")
	token := model.Token{AccessToken: "upload_file_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	var missingBody bytes.Buffer
	missingWriter := multipart.NewWriter(&missingBody)
	if err := missingWriter.WriteField("model", "slim"); err != nil {
		t.Fatal(err)
	}
	if err := missingWriter.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", &missingBody)
	req.Header.Set("Authorization", "Bearer upload_file_token")
	req.Header.Set("Content-Type", missingWriter.FormDataContentType())
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"file is required"`) {
		t.Fatalf("missing file upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	var nonPNGBody bytes.Buffer
	nonPNGWriter := multipart.NewWriter(&nonPNGBody)
	part, err := nonPNGWriter.CreateFormFile("file", "skin.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not a png")); err != nil {
		t.Fatal(err)
	}
	if err := nonPNGWriter.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", &nonPNGBody)
	req.Header.Set("Authorization", "Bearer upload_file_token")
	req.Header.Set("Content-Type", nonPNGWriter.FormDataContentType())
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec = httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Image must be PNG format"`) {
		t.Fatalf("non-PNG upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestTextureRoutesDeleteCapeClearsOnlyCape(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-delete-cape@test.com", "Password123", "YggDeleteCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_delete_cape_profile", "YggDeleteCapeProfile")
	skin := "skin_should_remain"
	cape := "cape_should_clear"
	if err := db.Profiles.UpdateSkin(context.Background(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(context.Background(), profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	token := model.Token{AccessToken: "delete_cape_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/cape", nil)
	req.Header.Set("Authorization", "Bearer delete_cape_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "cape")
	rec := httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete cape should be exact 204 empty body: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(context.Background(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skin || updated.CapeHash != nil {
		t.Fatalf("delete cape should clear only cape: profile=%#v err=%v", updated, err)
	}
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
