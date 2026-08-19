package yggdrasil_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

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
	skinData := testPNG(t, 64, 64)
	storage, err := texturesvc.NewTextureStorage(cfg.TexturesDir)
	if err != nil {
		t.Fatal(err)
	}
	hash, _, err := storage.ProcessAndSaveTracked(skinData, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, hash, "skin", "", false, "default"); err != nil {
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
	if _, err := part.Write(skinData); err != nil {
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
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != hash || updated.TextureModel != "slim" {
		t.Fatalf("upload should apply skin/model: profile=%#v err=%v", updated, err)
	}
}

func TestTextureUploadKeepsProfileHashAndModelAtomicWhenModelIsRejected(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload-atomic@test.com", "Password123", "YggUploadAtomic", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_atomic_profile", "YggUploadAtomic")
	token := model.Token{AccessToken: "upload_atomic_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(t.Context(), token, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(t.Context(), `
		ALTER TABLE profiles
		ADD CONSTRAINT reject_ygg_slim_model CHECK (texture_model <> 'slim')
	`); err != nil {
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
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)

	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("atomic model rejection response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Profiles.GetByID(t.Context(), profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash != nil || unchanged.TextureModel != "default" {
		t.Fatalf("failed upload apply changed profile hash/model: profile=%#v err=%v", unchanged, err)
	}
	page, err := db.Textures.ListForUser(t.Context(), user.ID, "skin", 10, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["model"] != "slim" {
		t.Fatalf("uploaded library row should remain after apply failure: %#v", page)
	}
}
