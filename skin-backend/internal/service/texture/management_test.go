package texture_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureManagementListUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	admin := testutil.CreateUser(t, db, "texture-admin-management@test.com", "Password123", "TextureAdminManagement", true)
	uploader := testutil.CreateUser(t, db, "texture-uploader-management@test.com", "Password123", "TextureUploaderManagement", false)
	wardrobeUser := testutil.CreateUser(t, db, "texture-wardrobe-management@test.com", "Password123", "TextureWardrobeManagement", false)
	actor := textureActor(admin.ID,
		"texture.read.any",
		"texture.update_metadata.any",
		"texture.update_visibility.any",
		"texture.delete.any",
	)
	if err := db.Textures.AddToLibrary(ctx, uploader.ID, "admin_manage_skin_a", "skin", "Manage A", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, uploader.ID, "admin_manage_skin_b", "skin", "Manage B", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, wardrobeUser.ID, "admin_manage_skin_a", "skin"); err != nil || !added {
		t.Fatalf("seed wardrobe copy: added=%v err=%v", added, err)
	}

	page, err := svc.ListAllTextures(ctx, actor, "", 1, "admin_manage", "skin")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if page["page_size"] != 1 || page["has_next"] != true || page["next_cursor"] == "" || len(items) != 1 {
		t.Fatalf("ListAllTextures page mismatch: %#v", page)
	}
	if items[0]["type"] != "skin" || items[0]["uploader_user_id"] != uploader.ID ||
		items[0]["uploader_email"] != uploader.Email || items[0]["uploader_display_name"] != uploader.DisplayName {
		t.Fatalf("ListAllTextures item metadata mismatch: %#v", items[0])
	}

	if err := svc.UpdateAnyTexture(ctx, actor, "admin_manage_skin_a", "skin", map[string]any{
		"note": "Managed A", "model": "default", "is_public": float64(0),
	}); err != nil {
		t.Fatal(err)
	}
	info, err := db.Textures.GetInfo(ctx, uploader.ID, "admin_manage_skin_a", "skin")
	if err != nil || info == nil || info["note"] != "Managed A" || info["model"] != "default" || info["is_public"] != 0 {
		t.Fatalf("updated texture info mismatch: info=%#v err=%v", info, err)
	}
	wardrobeInfo, err := db.Textures.GetInfo(ctx, wardrobeUser.ID, "admin_manage_skin_a", "skin")
	if err != nil || wardrobeInfo == nil || wardrobeInfo["note"] != "Managed A" || wardrobeInfo["model"] != "default" || wardrobeInfo["is_public"] != 2 {
		t.Fatalf("updated wardrobe texture info mismatch: info=%#v err=%v", wardrobeInfo, err)
	}

	if err := svc.UpdateAnyTexture(ctx, actor, "admin_manage_skin_a", "skin", map[string]any{"is_public": 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateAnyTexture(ctx, actor, "admin_manage_skin_a", "skin", map[string]any{"is_public": false}); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteAnyTexture(ctx, actor, "admin_manage_skin_a", "skin", wardrobeUser.ID, false); err != nil {
		t.Fatal(err)
	}
	if info, err := db.Textures.GetInfo(ctx, wardrobeUser.ID, "admin_manage_skin_a", "skin"); err != nil || info != nil {
		t.Fatalf("per-user admin delete should remove wardrobe row only: info=%#v err=%v", info, err)
	}
	if exists, err := db.Textures.Exists(ctx, "admin_manage_skin_a", "skin"); err != nil || !exists {
		t.Fatalf("per-user admin delete should retain library row: exists=%v err=%v", exists, err)
	}
	if err := svc.DeleteAnyTexture(ctx, actor, "admin_manage_skin_a", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := db.Textures.Exists(ctx, "admin_manage_skin_a", "skin"); err != nil || exists {
		t.Fatalf("forced admin delete should remove library row: exists=%v err=%v", exists, err)
	}
}

func TestTextureManagementRejectsInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	admin := testutil.CreateUser(t, db, "texture-admin-management-invalid@test.com", "Password123", "TextureAdminManagementInvalid", true)
	uploader := testutil.CreateUser(t, db, "texture-uploader-management-invalid@test.com", "Password123", "TextureUploaderManagementInvalid", false)
	readActor := textureActor(admin.ID, "texture.read.any")
	updateActor := textureActor(admin.ID, "texture.update_metadata.any", "texture.update_visibility.any")
	deleteActor := textureActor(admin.ID, "texture.delete.any")
	if err := db.Textures.AddToLibrary(ctx, uploader.ID, "admin_manage_invalid_skin", "skin", "Manage Invalid", true, "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListAllTextures(ctx, textureActor(admin.ID), "", 10, "", "skin"); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ListAllTextures without permission mismatch: %#v", err)
	}
	if _, err := svc.ListAllTextures(ctx, readActor, "bad-cursor", 10, "", "skin"); !httpErrorIs(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListAllTextures bad cursor mismatch: %#v", err)
	}
	if err := svc.UpdateAnyTexture(ctx, textureActor(admin.ID), "admin_manage_invalid_skin", "skin", map[string]any{"note": "Denied"}); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UpdateAnyTexture metadata permission mismatch: %#v", err)
	}
	if err := svc.UpdateAnyTexture(ctx, textureActor(admin.ID, "texture.update_metadata.any"), "admin_manage_invalid_skin", "skin", map[string]any{"is_public": true}); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UpdateAnyTexture visibility permission mismatch: %#v", err)
	}
	for _, tc := range []struct {
		name   string
		body   map[string]any
		detail string
	}{
		{"texture_model.validate.invalid", map[string]any{"model": "wide"}, "texture_model.validate.invalid"},
		{"invalid public bool", map[string]any{"is_public": "yes"}, "texture_visibility.validate.invalid"},
		{"empty body", map[string]any{}, "texture.update.required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.UpdateAnyTexture(ctx, updateActor, "admin_manage_invalid_skin", "skin", tc.body); !httpErrorIs(err, http.StatusBadRequest, tc.detail) {
				t.Fatalf("%s error mismatch: %#v", tc.name, err)
			}
		})
	}
	if err := svc.UpdateAnyTexture(ctx, updateActor, "admin_manage_invalid_skin", "elytra", map[string]any{"note": "bad"}); !httpErrorIs(err, http.StatusBadRequest, "texture_type.validate.invalid") {
		t.Fatalf("UpdateAnyTexture invalid type mismatch: %#v", err)
	}
	if err := svc.UpdateAnyTexture(ctx, updateActor, "missing-admin-manage", "skin", map[string]any{"note": "bad"}); !httpErrorIs(err, http.StatusNotFound, "texture.resolve.not_found") {
		t.Fatalf("UpdateAnyTexture missing texture mismatch: %#v", err)
	}
	if err := svc.DeleteAnyTexture(ctx, textureActor(admin.ID), "admin_manage_invalid_skin", "skin", uploader.ID, false); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("DeleteAnyTexture without permission mismatch: %#v", err)
	}
	if err := svc.DeleteAnyTexture(ctx, deleteActor, "admin_manage_invalid_skin", "skin", "", false); !httpErrorIs(err, http.StatusBadRequest, "texture.delete.required") {
		t.Fatalf("DeleteAnyTexture missing user_id mismatch: %#v", err)
	}
	if err := svc.DeleteAnyTexture(ctx, deleteActor, "missing-admin-manage", "skin", uploader.ID, false); !httpErrorIs(err, http.StatusNotFound, "texture.resolve.not_found") {
		t.Fatalf("DeleteAnyTexture missing texture mismatch: %#v", err)
	}
	if err := svc.DeleteAnyTexture(ctx, deleteActor, "missing-admin-manage", "skin", "", true); err != nil {
		t.Fatalf("forced DeleteAnyTexture should ignore missing library texture, got %#v", err)
	}
}
