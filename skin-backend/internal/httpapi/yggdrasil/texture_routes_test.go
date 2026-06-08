package yggdrasil_test

import (
	"context"
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
