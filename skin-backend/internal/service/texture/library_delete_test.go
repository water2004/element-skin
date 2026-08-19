package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUploaderDeleteRemovesWardrobeCopiesButKeepsAppliedProfileHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-delete-owner@test.com", "Password123", "DeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-delete-other@test.com", "Password123", "DeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_delete_profile", "SiteDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_delete_skin", "skin", "Texture Delete Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(other.ID), profile.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(owner.ID), "texture_service_delete_skin", "skin"); err != nil {
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
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-count-owner@test.com", "Password123", "CountOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-count-other@test.com", "Password123", "CountOther", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_count_skin", "skin", "Texture Count Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	public, err := svc.PublicLibrary(ctx, texturePublicActor(), "", 10, "skin", "Texture Count", "most_used")
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
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-missing-owner@test.com", "Password123", "MissingOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-missing-other@test.com", "Password123", "MissingOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_missing_delete_profile", "SiteMissingDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_missing_delete", "skin", "Missing Delete Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, ptrString("texture_service_missing_delete")); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteTexture(ctx, textureUserActor(other.ID), "texture_service_missing_delete", "skin")
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 404 || httpErr.Error() != "texture.resolve.not_found" {
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

func TestDeleteUserRecountsSharedLibraryButDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-profile-delete-owner@test.com", "Password123", "ProfileDeleteOwner", false)
	target := testutil.CreateUser(t, db, "site-profile-delete-target@test.com", "Password123", "ProfileDeleteTarget", false)
	other := testutil.CreateUser(t, db, "site-profile-delete-other@test.com", "Password123", "ProfileDeleteOther", false)

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(target.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "delete_user_uploaded_skin", "skin"); err != nil {
		t.Fatal(err)
	}

	accountSvc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	if err := accountSvc.DeleteUser(ctx, textureActor("admin-delete-user", "account.delete.any"), target.ID); err != nil {
		t.Fatalf("DeleteUser returned err=%v", err)
	}
	assertServicePublicUsage(t, svc, "delete_user_shared_skin", int64(2))
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}
