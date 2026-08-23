package texture_test

import (
	"context"
	"errors"
	"testing"

	profilesvc "element-skin/backend/internal/service/profile"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTexturesApplyUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-textures-service@test.com", "Password123", "SiteTexturesService", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_textures_profile", "SiteTexturesProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_skin", "skin", "Texture Service Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updatedProfile.SkinHash == nil || *updatedProfile.SkinHash != "texture_service_skin" || updatedProfile.TextureModel != "slim" {
		t.Fatalf("profile texture state mismatch: profile=%#v err=%v", updatedProfile, err)
	}
	detail, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "texture_service_skin", "skin", map[string]any{"note": "Updated Texture Service", "is_public": false})
	if err != nil || detail["note"] != "Updated Texture Service" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: detail=%#v err=%v", detail, err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(user.ID), "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if info, err := db.Textures.GetInfo(ctx, user.ID, "texture_service_skin", "skin"); err != nil || info != nil {
		t.Fatalf("texture should be deleted: info=%#v err=%v", info, err)
	}
}

func TestApplyTextureRejectsMissingForeignAndInvalidTypeWithoutMutatingProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	profileSvc := profilesvc.Service{DB: db}
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
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), profile.ID, "missing_apply_texture", "skin")
		}, 403, "texture.authorize.denied"},
		{"foreign profile", func() error {
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), foreign.ID, "texture_service_apply_skin", "skin")
		}, 403, "profile.authorize.denied"},
		{"type.validate.invalid", func() error {
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), profile.ID, "texture_service_apply_skin", "elytra")
		}, 403, "texture.authorize.denied"},
		{"set invalid type", func() error {
			return profileSvc.SetProfileTexture(ctx, textureActor("texture-service-admin", "profile.update.any"), profile.ID, "elytra", ptrString("texture_service_apply_skin"))
		}, 400, "texture_type.validate.invalid"},
		{"set missing profile", func() error {
			return profileSvc.SetProfileTexture(ctx, textureActor("texture-service-admin", "profile.update.any"), "missing-profile", "skin", ptrString("texture_service_apply_skin"))
		}, 404, "profile.resolve.not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpErrorIs(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash != nil || unchanged.CapeHash != nil || unchanged.TextureModel != profile.TextureModel {
		t.Fatalf("failed apply attempts must not mutate profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestApplySkinRollsBackHashWhenModelUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
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

	err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "site_apply_atomic_skin", "skin")
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

func TestTextureServiceAppliesCapeWithoutChangingSkinOrModel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-cape@test.com", "Password123", "TextureCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_cape_profile", "TextureCapeProfile")
	skin := "existing_skin_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_cape", "cape", "Texture Service Cape", true, "slim"); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "texture_service_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skin ||
		updated.CapeHash == nil || *updated.CapeHash != "texture_service_cape" ||
		updated.TextureModel != profile.TextureModel {
		t.Fatalf("cape apply must change only cape hash: profile=%#v err=%v", updated, err)
	}
}
