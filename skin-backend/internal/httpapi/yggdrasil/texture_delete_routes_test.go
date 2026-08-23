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
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("upload without bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/skin", nil)
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("delete without bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	profileID := profile.ID
	if err := redis.SetYggToken(context.Background(), model.Token{AccessToken: "delete_texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: time.Now().UnixMilli()}, time.Minute); err != nil {
		t.Fatal(err)
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

func TestTextureDeleteRejectsTokenAfterProfileIDIsReassigned(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	originalOwner := testutil.CreateUser(t, db, "ygg-route-stale-owner@test.com", "Password123", "YggRouteStaleOwner", false)
	newOwner := testutil.CreateUser(t, db, "ygg-route-new-owner@test.com", "Password123", "YggRouteNewOwner", false)
	profile := testutil.CreateProfile(t, db, originalOwner.ID, "ygg_route_reassigned", "YggRouteOriginal")
	token := model.Token{
		AccessToken: "stale_route_access",
		ClientToken: "stale_route_client",
		UserID:      originalOwner.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   time.Now().UnixMilli(),
	}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Profiles.DeleteCascade(context.Background(), profile.ID); err != nil || !ok {
		t.Fatalf("delete original profile: ok=%v err=%v", ok, err)
	}
	skin := "new_owner_skin_must_remain"
	if err := db.Profiles.Create(context.Background(), model.Profile{
		ID:           profile.ID,
		UserID:       newOwner.ID,
		Name:         "YggRouteReassigned",
		TextureModel: "slim",
		SkinHash:     &skin,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/skin", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":\"Unauthorized\",\"errorMessage\":\"Invalid token\"}\n" {
		t.Fatalf("stale reassigned-profile token response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Profiles.GetByID(context.Background(), profile.ID)
	if err != nil || unchanged == nil || unchanged.UserID != newOwner.ID || unchanged.SkinHash == nil || *unchanged.SkinHash != skin {
		t.Fatalf("rejected stale token must preserve the new owner's skin: profile=%#v err=%v", unchanged, err)
	}
}
