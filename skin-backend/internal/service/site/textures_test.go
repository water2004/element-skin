package site_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestTexturesApplyUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-textures-service@test.com", "Password123", "SiteTexturesService", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_textures_profile", "SiteTexturesProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_skin", "skin", "Texture Service Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, user.ID, profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
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
	if info, err := db.Textures.GetInfo(ctx, user.ID, "texture_service_skin", "skin"); err != nil || info != nil {
		t.Fatalf("texture should be deleted: info=%#v err=%v", info, err)
	}
}

func TestApplyTextureRejectsMissingForeignAndInvalidTypeWithoutMutatingProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-apply-owner@test.com", "Password123", "ApplyOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-apply-other@test.com", "Password123", "ApplyOther", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "site_apply_profile", "SiteApplyProfile")
	foreign := testutil.CreateProfile(t, db, other.ID, "site_apply_foreign", "SiteApplyForeign")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_apply_skin", "skin", "Texture Apply Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		call func() error
		code int
		want string
	}{
		{"missing texture ownership", func() error {
			return svc.ApplyTextureToProfile(ctx, owner.ID, profile.ID, "missing_apply_texture", "skin")
		}, 403, "Texture not found in your library"},
		{"foreign profile", func() error {
			return svc.ApplyTextureToProfile(ctx, owner.ID, foreign.ID, "texture_service_apply_skin", "skin")
		}, 403, "Profile not yours"},
		{"invalid type", func() error {
			return svc.ApplyTextureToProfile(ctx, owner.ID, profile.ID, "texture_service_apply_skin", "elytra")
		}, 403, "Texture not found in your library"},
		{"set invalid type", func() error {
			return svc.SetProfileTexture(ctx, profile.ID, "elytra", ptrString("texture_service_apply_skin"))
		}, 400, "Invalid texture_type"},
		{"set missing profile", func() error {
			return svc.SetProfileTexture(ctx, "missing-profile", "skin", ptrString("texture_service_apply_skin"))
		}, 404, "profile not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpError(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash != nil || unchanged.CapeHash != nil || unchanged.TextureModel != profile.TextureModel {
		t.Fatalf("failed apply attempts must not mutate profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestUploaderDeleteRemovesWardrobeCopiesButKeepsAppliedProfileHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-delete-owner@test.com", "Password123", "DeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-delete-other@test.com", "Password123", "DeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_delete_profile", "SiteDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_delete_skin", "skin", "Texture Delete Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, other.ID, profile.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, owner.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if exists, err := db.Textures.Exists(ctx, "texture_service_delete_skin", "skin"); err != nil || exists {
		t.Fatalf("uploader delete should remove skin_library row: exists=%v err=%v", exists, err)
	}
	for _, userID := range []string{owner.ID, other.ID} {
		if info, err := db.Textures.GetInfo(ctx, userID, "texture_service_delete_skin", "skin"); err != nil || info != nil {
			t.Fatalf("uploader delete should remove personal library row for %s: info=%#v err=%v", userID, info, err)
		}
	}
	afterDelete, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || afterDelete == nil || afterDelete.SkinHash == nil || *afterDelete.SkinHash != "texture_service_delete_skin" {
		t.Fatalf("applied profile hash should remain until user clears it: profile=%#v err=%v", afterDelete, err)
	}
}

func TestNonUploaderDeleteOnlyDecrementsUsageCount(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-count-owner@test.com", "Password123", "CountOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-count-other@test.com", "Password123", "CountOther", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_count_skin", "skin", "Texture Count Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, other.ID, "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	public, err := svc.PublicLibrary(ctx, "", 10, "skin", "Texture Count", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["usage_count"] != int64(1) {
		t.Fatalf("non-uploader delete should leave owner count only: %#v", public)
	}
	if exists, err := db.Textures.Exists(ctx, "texture_service_count_skin", "skin"); err != nil || !exists {
		t.Fatalf("non-uploader delete should keep library row: exists=%v err=%v", exists, err)
	}
}

func TestDeleteMissingWardrobeTextureReturnsNotFoundAndKeepsAppliedHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-missing-owner@test.com", "Password123", "MissingOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-missing-other@test.com", "Password123", "MissingOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_missing_delete_profile", "SiteMissingDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_missing_delete", "skin", "Missing Delete Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, ptrString("texture_service_missing_delete")); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteTexture(ctx, other.ID, "texture_service_missing_delete", "skin")
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 404 || httpErr.Detail != "Texture not found" {
		t.Fatalf("missing wardrobe delete should return exact 404 error, got %#v", err)
	}
	if info, err := db.Textures.GetInfo(ctx, owner.ID, "texture_service_missing_delete", "skin"); err != nil || info == nil {
		t.Fatalf("missing wardrobe delete must keep uploader library row: info=%#v err=%v", info, err)
	}
	afterDelete, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || afterDelete == nil || afterDelete.SkinHash == nil || *afterDelete.SkinHash != "texture_service_missing_delete" {
		t.Fatalf("missing wardrobe delete must not clear applied profile hash: profile=%#v err=%v", afterDelete, err)
	}
}

func TestTextureServiceMapsMissingUpdatesAndDetailToExactNotFound(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-texture-missing-update@test.com", "Password123", "TextureMissingUpdate", false)

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{name: "detail", call: func() error {
			_, err := svc.TextureDetail(ctx, user.ID, "missing_texture", "skin")
			return err
		}},
		{name: "note update", call: func() error {
			_, err := svc.UpdateTexture(ctx, user.ID, "missing_texture", "skin", map[string]any{"note": "No row"})
			return err
		}},
		{name: "model update", call: func() error {
			_, err := svc.UpdateTexture(ctx, user.ID, "missing_texture", "skin", map[string]any{"model": "slim"})
			return err
		}},
		{name: "visibility update", call: func() error {
			_, err := svc.UpdateTexture(ctx, user.ID, "missing_texture", "skin", map[string]any{"is_public": true})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !httpError(err, 404, "Texture not found") {
				t.Fatalf("%s should map to exact not-found error, got %#v", tc.name, err)
			}
		})
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("missing texture operations must not create rows: count=%d err=%v", count, err)
	}
}

func TestTextureServiceAppliesCapeWithoutChangingSkinOrModel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-texture-cape@test.com", "Password123", "TextureCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_cape_profile", "TextureCapeProfile")
	skin := "existing_skin_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_cape", "cape", "Texture Service Cape", true, "slim"); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyTextureToProfile(ctx, user.ID, profile.ID, "texture_service_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skin ||
		updated.CapeHash == nil || *updated.CapeHash != "texture_service_cape" ||
		updated.TextureModel != profile.TextureModel {
		t.Fatalf("cape apply must change only cape hash: profile=%#v err=%v", updated, err)
	}
}

func ptrString(s string) *string {
	return &s
}
