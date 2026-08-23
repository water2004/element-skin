package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpdateTextureRejectsInvalidFieldsBeforeMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-validate@test.com", "Password123", "TextureValidate", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_validate", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		body   map[string]any
		detail string
	}{
		{"texture_model.validate.invalid", map[string]any{"note": "Changed", "model": "wide"}, "texture_model.validate.invalid"},
		{"invalid public", map[string]any{"note": "Changed", "is_public": "yes"}, "texture_visibility.validate.invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "site_texture_validate", "skin", test.body)
			if result != nil || !httpErrorIs(err, 400, test.detail) {
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
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-patch-rollback@test.com", "Password123", "TexturePatchRollback", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_patch_rollback", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE skin_library ADD CONSTRAINT reject_slim_library_model CHECK (model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	result, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "site_texture_patch_rollback", "skin", map[string]any{
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
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-missing-update@test.com", "Password123", "TextureMissingUpdate", false)

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{name: "detail", call: func() error {
			_, err := svc.TextureDetail(ctx, textureUserActor(user.ID), "missing_texture", "skin")
			return err
		}},
		{name: "note update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"note": "No row"})
			return err
		}},
		{name: "model update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"model": "slim"})
			return err
		}},
		{name: "visibility update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"is_public": true})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !httpErrorIs(err, 404, "texture.resolve.not_found") {
				t.Fatalf("%s should map to exact not-found error, got %#v", tc.name, err)
			}
		})
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("missing texture operations must not create rows: count=%d err=%v", count, err)
	}
}
