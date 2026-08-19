package yggdrasil_test

import (
	"bytes"
	"context"
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
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"access_token\",\"operation\":\"verify\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("profile mismatch should be generic 401 invalid token: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/elytra", nil)
	req.Header.Set("Authorization", "Bearer texture_rules_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "elytra")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_type\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
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
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_type\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
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
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"access_token\",\"operation\":\"verify\",\"reason\":\"invalid\"}}\n" {
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
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
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
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"upload_file\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
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
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"texture_format\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n" {
		t.Fatalf("non-PNG upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
