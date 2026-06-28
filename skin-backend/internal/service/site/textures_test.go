package site_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
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
	if err := svc.ApplyTextureToProfile(ctx, testUserActor(user.ID), profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updatedProfile.SkinHash == nil || *updatedProfile.SkinHash != "texture_service_skin" || updatedProfile.TextureModel != "slim" {
		t.Fatalf("profile texture state mismatch: profile=%#v err=%v", updatedProfile, err)
	}
	detail, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "texture_service_skin", "skin", map[string]any{"note": "Updated Texture Service", "is_public": false})
	if err != nil || detail["note"] != "Updated Texture Service" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: detail=%#v err=%v", detail, err)
	}
	if err := svc.DeleteTexture(ctx, testUserActor(user.ID), "texture_service_skin", "skin"); err != nil {
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
			return svc.ApplyTextureToProfile(ctx, testUserActor(owner.ID), profile.ID, "missing_apply_texture", "skin")
		}, 403, "Texture not found in your library"},
		{"foreign profile", func() error {
			return svc.ApplyTextureToProfile(ctx, testUserActor(owner.ID), foreign.ID, "texture_service_apply_skin", "skin")
		}, 403, "Profile not yours"},
		{"invalid type", func() error {
			return svc.ApplyTextureToProfile(ctx, testUserActor(owner.ID), profile.ID, "texture_service_apply_skin", "elytra")
		}, 403, "Texture not found in your library"},
		{"set invalid type", func() error {
			return svc.SetProfileTexture(ctx, testActorWithCodes("texture-service-admin", "profile.update.any"), profile.ID, "elytra", ptrString("texture_service_apply_skin"))
		}, 400, "Invalid texture_type"},
		{"set missing profile", func() error {
			return svc.SetProfileTexture(ctx, testActorWithCodes("texture-service-admin", "profile.update.any"), "missing-profile", "skin", ptrString("texture_service_apply_skin"))
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
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(other.ID), "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, testUserActor(other.ID), profile.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, testUserActor(owner.ID), "texture_service_delete_skin", "skin"); err != nil {
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
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, testUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
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

	err := svc.DeleteTexture(ctx, testUserActor(other.ID), "texture_service_missing_delete", "skin")
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

func TestApplySkinRollsBackHashWhenModelUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-apply-atomic@test.com", "Password123", "ApplyAtomic", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_apply_atomic", "SiteApplyAtomic")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_apply_atomic_skin", "skin", "Atomic Skin", false, "slim"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE profiles ADD CONSTRAINT reject_slim_model CHECK (texture_model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	err := svc.ApplyTextureToProfile(ctx, testUserActor(user.ID), profile.ID, "site_apply_atomic_skin", "skin")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("apply skin failure = %#v, want PostgreSQL 23514", err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil ||
		unchanged.SkinHash != nil ||
		unchanged.TextureModel != "default" {
		t.Fatalf("failed skin apply must preserve hash and model: profile=%#v err=%v", unchanged, err)
	}
}

func TestUpdateTextureRejectsInvalidFieldsBeforeMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-texture-validate@test.com", "Password123", "TextureValidate", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_validate", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		body   map[string]any
		detail string
	}{
		{"invalid model", map[string]any{"note": "Changed", "model": "wide"}, "invalid model"},
		{"invalid public", map[string]any{"note": "Changed", "is_public": "yes"}, "invalid is_public"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "site_texture_validate", "skin", test.body)
			if result != nil || !httpError(err, 400, test.detail) {
				t.Fatalf("invalid update result=%#v err=%#v, want exact 400 %q", result, err, test.detail)
			}
			info, err := db.Textures.GetInfo(ctx, user.ID, "site_texture_validate", "skin")
			if err != nil || info == nil ||
				info["note"] != "Original" ||
				info["model"] != "default" ||
				info["is_public"] != 1 {
				t.Fatalf("invalid update changed texture: info=%#v err=%v", info, err)
			}
		})
	}
}

func TestUpdateTextureRollsBackAllFieldsWhenLibraryModelUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-texture-patch-rollback@test.com", "Password123", "TexturePatchRollback", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_patch_rollback", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE skin_library ADD CONSTRAINT reject_slim_library_model CHECK (model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	result, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "site_texture_patch_rollback", "skin", map[string]any{
		"note":      "Changed",
		"model":     "slim",
		"is_public": false,
	})
	var pgErr *pgconn.PgError
	if result != nil || !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("atomic user patch result=%#v err=%#v, want nil and PostgreSQL 23514", result, err)
	}
	info, err := db.Textures.GetInfo(ctx, user.ID, "site_texture_patch_rollback", "skin")
	if err != nil || info == nil ||
		info["note"] != "Original" ||
		info["model"] != "default" ||
		info["is_public"] != 1 {
		t.Fatalf("failed user patch changed texture: info=%#v err=%v", info, err)
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
			_, err := svc.TextureDetail(ctx, testUserActor(user.ID), "missing_texture", "skin")
			return err
		}},
		{name: "note update", call: func() error {
			_, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "missing_texture", "skin", map[string]any{"note": "No row"})
			return err
		}},
		{name: "model update", call: func() error {
			_, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "missing_texture", "skin", map[string]any{"model": "slim"})
			return err
		}},
		{name: "visibility update", call: func() error {
			_, err := svc.UpdateTexture(ctx, testUserActor(user.ID), "missing_texture", "skin", map[string]any{"is_public": true})
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

	if err := svc.ApplyTextureToProfile(ctx, testUserActor(user.ID), profile.ID, "texture_service_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skin ||
		updated.CapeHash == nil || *updated.CapeHash != "texture_service_cape" ||
		updated.TextureModel != profile.TextureModel {
		t.Fatalf("cape apply must change only cape hash: profile=%#v err=%v", updated, err)
	}
}

func TestApplyTextureToProfileWithModel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "apply-model@test.com", "Password123", "ApplyModel", false)
	profile := testutil.CreateProfile(t, db, user.ID, "apply_model_profile", "ApplyModelProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "apply_model_skin", "skin", "Model Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyTextureToProfileWithModel(ctx, testUserActor(user.ID), profile.ID, "apply_model_skin", "skin", "default"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil {
		t.Fatalf("profile not found after apply: err=%v", err)
	}
	if updated.SkinHash == nil || *updated.SkinHash != "apply_model_skin" {
		t.Fatalf("skin hash mismatch: %#v", updated.SkinHash)
	}
	if updated.TextureModel != "default" {
		t.Fatalf("model should be 'default' as explicitly passed: %s", updated.TextureModel)
	}
}

func ptrString(s string) *string {
	return &s
}
