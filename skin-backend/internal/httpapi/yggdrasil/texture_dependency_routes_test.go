package yggdrasil_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestTextureUploadReturnsExactDependencyErrors(t *testing.T) {
	t.Run("permission actor", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cfg.TexturesDir = t.TempDir()
		redis := testutil.NewMemoryRedis()
		h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
		user := testutil.CreateUser(t, db, "ygg-upload-permission-fail@test.com", "Password123", "YggUploadPermissionFail", false)
		profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_permission_fail", "YggUploadPermissionFail")
		token := model.Token{AccessToken: "ygg_upload_permission_fail_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
		if err := redis.SetYggToken(t.Context(), token, time.Hour); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(t.Context(), `ALTER TABLE permission_subjects RENAME TO permission_subjects_unavailable`); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", strings.NewReader("body is never parsed"))
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.SetPathValue("uuid", profile.ID)
		req.SetPathValue("texture_type", "skin")
		rec := httptest.NewRecorder()
		h.UploadTexture(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("permission dependency response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("storage", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		blocker := filepath.Join(t.TempDir(), "textures")
		if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg.TexturesDir = blocker
		redis := testutil.NewMemoryRedis()
		h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
		user := testutil.CreateUser(t, db, "ygg-upload-storage-fail@test.com", "Password123", "YggUploadStorageFail", false)
		profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_storage_fail", "YggUploadStorageFail")
		token := model.Token{AccessToken: "ygg_upload_storage_fail_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
		if err := redis.SetYggToken(t.Context(), token, time.Hour); err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
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
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.SetPathValue("uuid", profile.ID)
		req.SetPathValue("texture_type", "skin")
		rec := httptest.NewRecorder()
		h.UploadTexture(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("storage dependency response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if count, err := db.Textures.CountForUser(t.Context(), user.ID); err != nil || count != 0 {
			t.Fatalf("storage failure must not create texture rows: count=%d err=%v", count, err)
		}
	})
}

func TestYggTextureUploadRemovesNewFileWhenDatabaseInsertFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	user := testutil.CreateUser(t, db, "ygg-upload-db-fail@test.com", "Password123", "YggUploadDBFail", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_db_fail", "YggUploadDBFail")
	token := model.Token{AccessToken: "ygg_upload_db_fail_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(t.Context(), token, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(t.Context(), `ALTER TABLE user_textures ADD CONSTRAINT reject_ygg_test_upload CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
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
		t.Fatalf("ygg database upload failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if count, err := db.Textures.CountForUser(t.Context(), user.ID); err != nil || count != 0 {
		t.Fatalf("failed ygg database insert must leave no texture row: count=%d err=%v", count, err)
	}
	entries, err := os.ReadDir(cfg.TexturesDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed ygg database insert must remove the newly-created texture file: entries=%#v err=%v", entries, err)
	}
	unchanged, err := db.Profiles.GetByID(t.Context(), profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash != nil {
		t.Fatalf("failed ygg database insert must not apply a skin: profile=%#v err=%v", unchanged, err)
	}
}
