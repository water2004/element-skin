package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestTexturesApplyUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := site.Site{DB: db, Cfg: testutil.TestConfig()}
	user := testutil.CreateUser(t, db, "site-textures-service@test.com", "Password123", "SiteTexturesService", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_textures_profile", "SiteTexturesProfile")
	if err := db.AddTextureToLibrary(ctx, user.ID, "texture_service_skin", "skin", "Texture Service Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, user.ID, profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.GetProfileByID(ctx, profile.ID)
	if err != nil || updatedProfile.SkinHash == nil || *updatedProfile.SkinHash != "texture_service_skin" || updatedProfile.TextureModel != "slim" {
		t.Fatalf("profile texture state mismatch: profile=%#v err=%v", updatedProfile, err)
	}
	detail, err := svc.UpdateTexture(ctx, user.ID, "texture_service_skin", "skin", map[string]any{"note": "Updated Texture Service", "is_public": false})
	if err != nil || detail["note"] != "Updated Texture Service" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: detail=%#v err=%v", detail, err)
	}
	if err := svc.DeleteTexture(ctx, user.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if info, err := db.GetTextureInfo(ctx, user.ID, "texture_service_skin", "skin"); err != nil || info != nil {
		t.Fatalf("texture should be deleted: info=%#v err=%v", info, err)
	}
}
