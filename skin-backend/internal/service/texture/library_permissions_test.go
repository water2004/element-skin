package texture_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
)

func TestTexturePermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := texturesvc.LibraryService{DB: db}
	user := testutil.CreateUser(t, db, "perm-texture-deny@test.com", "Password123", "PermTextureDeny", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_texture_deny", "PermTextureDeny")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "perm_texture_skin", "skin", "Perm Texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		actorCode string
		call      func(permission.Actor) error
		status    int
		detail    string
	}{
		{
			name:      "TextureDetail without texture.read.owned",
			actorCode: "texture.create.owned",
			call: func(a permission.Actor) error {
				_, err := svc.TextureDetail(ctx, a, "perm_texture_skin", "skin")
				return err
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "ListMyTextures without texture.read.owned",
			actorCode: "texture.create.owned",
			call: func(a permission.Actor) error {
				_, err := svc.ListMyTextures(ctx, a, "", 10, "skin")
				return err
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "DeleteTexture without texture.delete.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.DeleteTexture(ctx, a, "perm_texture_skin", "skin")
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "AddTextureToWardrobe without wardrobe_entry.add.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.AddTextureToWardrobe(ctx, a, "perm_texture_skin", "skin")
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "ApplyTextureToProfile without texture.apply.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.ApplyTextureToProfile(ctx, a, profile.ID, "perm_texture_skin", "skin")
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "UpdateTexture note without texture.update_metadata.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				_, err := svc.UpdateTexture(ctx, a, "perm_texture_skin", "skin", map[string]any{"note": "new"})
				return err
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
		{
			name:      "UpdateTexture is_public without texture.update_visibility.owned",
			actorCode: "texture.update_metadata.owned",
			call: func(a permission.Actor) error {
				_, err := svc.UpdateTexture(ctx, a, "perm_texture_skin", "skin", map[string]any{"is_public": true})
				return err
			},
			status: http.StatusForbidden,
			detail: "permission.check.denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(textureActor(user.ID, tc.actorCode))
			if !httpErrorIs(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

func TestApplyTextureWithBoundActor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := texturesvc.LibraryService{DB: db}
	user := testutil.CreateUser(t, db, "perm-apply-bound@test.com", "Password123", "PermApplyBound", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_apply_bound", "PermApplyBound")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "bound_apply_skin", "skin", "Bound Apply", false, "default"); err != nil {
		t.Fatal(err)
	}

	actor := textureActor(user.ID, "texture.apply.bound_profile")
	actor.BoundProfileID = profile.ID
	if err := svc.ApplyTextureToProfile(ctx, actor, profile.ID, "bound_apply_skin", "skin"); err != nil {
		t.Fatalf("ApplyTextureToProfile with bound_profile scope should succeed, got %#v", err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != "bound_apply_skin" {
		t.Fatalf("skin should be applied: profile=%#v err=%v", updated, err)
	}
}

func TestApplyTextureBoundMismatch(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := texturesvc.LibraryService{DB: db}
	owner := testutil.CreateUser(t, db, "perm-bound-mismatch@test.com", "Password123", "PermBoundMismatch", false)
	profile1 := testutil.CreateProfile(t, db, owner.ID, "perm_bound_mismatch_1", "BoundMismatch1")
	profile2 := testutil.CreateProfile(t, db, owner.ID, "perm_bound_mismatch_2", "BoundMismatch2")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "bound_mismatch_skin", "skin", "Mismatch", false, "default"); err != nil {
		t.Fatal(err)
	}

	actor := textureActor(owner.ID, "texture.apply.bound_profile")
	actor.BoundProfileID = profile1.ID
	err := svc.ApplyTextureToProfile(ctx, actor, profile2.ID, "bound_mismatch_skin", "skin")
	if !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("bound actor applying to wrong profile should be denied, got %#v", err)
	}
}
