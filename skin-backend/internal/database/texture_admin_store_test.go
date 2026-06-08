package database_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestTextureAdminStoreUpdateListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-admin-store@test.com", "Password123", "TextureAdminStore", false)
	if err := db.AddTextureToLibrary(ctx, user.ID, "texture_admin_hash", "skin", "Admin Texture", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.AdminUpdateTextureNote(ctx, "texture_admin_hash", "Admin Updated"); err != nil {
		t.Fatal(err)
	}
	if err := db.AdminUpdateTextureModel(ctx, "texture_admin_hash", "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AdminUpdateTexturePublic(ctx, "texture_admin_hash", false); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListAllTextures(ctx, 1, nil, "", "Admin Updated", "skin")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "texture_admin_hash" || items[0]["name"] != "Admin Updated" || items[0]["model"] != "default" || items[0]["is_public"] != false {
		t.Fatalf("admin texture list mismatch: %#v", list)
	}
	if err := db.AdminDeleteTexture(ctx, "texture_admin_hash", "skin", "", false); err == nil || err.Error() != "per-user deletion requires user_id" {
		if err == nil || err.Error() != "per-user deletion requires user_id" {
			t.Fatalf("per-user delete without user_id should reject exactly, err=%v", err)
		}
	}
	if err := db.AdminDeleteTexture(ctx, "texture_admin_hash", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := db.TextureExists(ctx, "texture_admin_hash"); err != nil || exists {
		t.Fatalf("force delete should remove texture: exists=%v err=%v", exists, err)
	}
	if err := db.AdminUpdateTextureNote(ctx, "missing_hash", "note"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("missing texture should return ErrNotFound, got %v", err)
	}
}
