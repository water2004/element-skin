package integration_test

import (
	"context"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"testing"
	"time"
)

func TestSelfDeleteAndDirectTextureUploadHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "selfdelete@test.com", "Password123", "SelfDelete", false)
	profile := testutil.CreateProfile(t, db, user.ID, "direct_upload_profile", "DirectUpload")
	access, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: access}

	direct := doMultipart(t, h, "POST", "/v2/users/me/textures/upload-and-apply", map[string]string{
		"uuid":         profile.ID,
		"texture_type": "skin",
		"model":        "slim",
		"is_public":    "false",
	}, "file", "skin.png", pngTexture(t, 64, 64), cookie)
	if direct.Code != http.StatusCreated {
		t.Fatalf("direct upload status=%d body=%s", direct.Code, direct.Body.String())
	}
	updated, _ := db.Profiles.GetByID(context.Background(), profile.ID)
	if updated.SkinHash == nil || updated.TextureModel != "slim" {
		t.Fatalf("direct upload did not apply texture/model: %#v", updated)
	}

	if err := db.Tokens.AddRefresh(context.Background(), "self_delete_refresh", user.ID, database.NowMS()+3600*1000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	del := doJSON(t, h, "DELETE", "/v2/users/me", nil, cookie)
	if del.Code != http.StatusNoContent || del.Body.Len() != 0 {
		t.Fatalf("self delete status=%d body=%s", del.Code, del.Body.String())
	}
	if row, _ := db.Users.GetByID(context.Background(), user.ID); row != nil {
		t.Fatal("self delete should remove user")
	}
	if row, _ := db.Tokens.GetRefresh(context.Background(), "self_delete_refresh"); row != nil {
		t.Fatal("self delete should revoke refresh tokens")
	}

	admin := testutil.CreateUser(t, db, "selfadmin@test.com", "Password123", "SelfAdmin", true, true)
	adminAccess, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, time.Hour)
	adminDel := doJSON(t, h, "DELETE", "/v2/users/me", nil, &http.Cookie{Name: "access_token", Value: adminAccess})
	if adminDel.Code != 403 {
		t.Fatalf("admin self delete should be 403, got %d", adminDel.Code)
	}
}
