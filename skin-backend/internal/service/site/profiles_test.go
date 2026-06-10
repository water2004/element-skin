package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, user.ID, "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, user.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}
}

func TestProfilesRejectInvalidProfileAndLibraryInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profile-invalid@test.com", "Password123", "ProfileInvalid", false)
	other := testutil.CreateUser(t, db, "site-profile-invalid-other@test.com", "Password123", "ProfileInvalidOther", false)
	existing := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_existing", "ExistingSvc")
	target := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_target", "TargetSvc")
	foreign := testutil.CreateProfile(t, db, other.ID, "site_profile_invalid_foreign", "ForeignSvc")

	for _, tc := range []struct {
		name string
		call func() error
		code int
		want string
	}{
		{"empty create name", func() error {
			_, err := svc.CreateProfile(ctx, user.ID, "", "default")
			return err
		}, 400, "name required"},
		{"invalid create name", func() error {
			_, err := svc.CreateProfile(ctx, user.ID, "bad-name!", "default")
			return err
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate create name", func() error {
			_, err := svc.CreateProfile(ctx, user.ID, existing.Name, "default")
			return err
		}, 400, "角色名已被占用，请换一个名称"},
		{"empty update name", func() error {
			return svc.UpdateProfile(ctx, user.ID, target.ID, "")
		}, 400, "name required"},
		{"invalid update name", func() error {
			return svc.UpdateProfile(ctx, user.ID, target.ID, "bad-name!")
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate update name", func() error {
			return svc.UpdateProfile(ctx, user.ID, target.ID, existing.Name)
		}, 400, "角色名已被占用"},
		{"foreign update", func() error {
			return svc.UpdateProfile(ctx, user.ID, foreign.ID, "StolenProfileSvc")
		}, 403, "not allowed"},
		{"missing update", func() error {
			return svc.UpdateProfile(ctx, user.ID, "missing-profile", "MissingProfileSvc")
		}, 404, "profile not found"},
		{"foreign delete", func() error {
			return svc.DeleteProfile(ctx, user.ID, foreign.ID)
		}, 403, "not allowed"},
		{"missing delete", func() error {
			return svc.DeleteProfile(ctx, user.ID, "missing-profile")
		}, 404, "profile not found"},
		{"invalid clear texture type", func() error {
			return svc.ClearProfileTexture(ctx, user.ID, target.ID, "elytra")
		}, 400, "Invalid texture_type"},
		{"foreign clear texture", func() error {
			return svc.ClearProfileTexture(ctx, user.ID, foreign.ID, "skin")
		}, 403, "not allowed"},
		{"missing clear texture", func() error {
			return svc.ClearProfileTexture(ctx, user.ID, "missing-profile", "skin")
		}, 404, "profile not found"},
		{"missing wardrobe add", func() error {
			return svc.AddTextureToWardrobe(ctx, user.ID, "missing_texture_hash", "skin")
		}, 404, "Texture not found in library"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpError(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, target.ID)
	if err != nil || unchanged == nil || unchanged.Name != target.Name {
		t.Fatalf("invalid profile mutations should not change target: profile=%#v err=%v", unchanged, err)
	}
	foreignAfter, err := db.Profiles.GetByID(ctx, foreign.ID)
	if err != nil || foreignAfter == nil || foreignAfter.Name != foreign.Name {
		t.Fatalf("foreign profile should remain unchanged: profile=%#v err=%v", foreignAfter, err)
	}
}

func TestProfilesCursorsDisabledLibraryAndAdminDeleteByID(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_profile_admin_delete", "AdminDeleteProfileSvc")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "profile_cursor_skin", "skin", "Profile Cursor Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListMyProfiles(ctx, user.ID, "not-base64", 10); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid profile cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.ListMyTextures(ctx, user.ID, "not-base64", 10, "skin"); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid texture cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.PublicLibrary(ctx, "not-base64", 10, "skin", "", "latest"); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid public library cursor should reject exactly, got %#v", err)
	}

	if err := db.Settings.Set(ctx, "enable_skin_library", "false"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicLibrary(ctx, "", 10, "skin", "", "latest"); !httpError(err, 403, "Skin library is disabled by administrator") {
		t.Fatalf("disabled public library should reject exactly, got %#v", err)
	}

	if err := svc.DeleteProfileByID(ctx, profile.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfileByID should remove profile regardless of owner: profile=%#v err=%v", got, err)
	}
	if err := svc.DeleteProfileByID(ctx, profile.ID); !httpError(err, 404, "profile not found") {
		t.Fatalf("DeleteProfileByID missing profile should reject exactly, got %#v", err)
	}
}

func TestProfilesListTexturesParsesCursorAndPublicLibraryMostUsedCursor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-profile-cursor-owner@test.com", "Password123", "ProfileCursorOwner", false)
	other := testutil.CreateUser(t, db, "site-profile-cursor-other@test.com", "Password123", "ProfileCursorOther", false)
	for _, item := range []struct {
		hash string
		name string
	}{
		{"profile_cursor_old", "Profile Cursor Old"},
		{"profile_cursor_new", "Profile Cursor New"},
	} {
		if err := db.Textures.AddToLibrary(ctx, owner.ID, item.hash, "skin", item.name, true, "default"); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "profile_cursor_old", "skin"); err != nil {
		t.Fatal(err)
	}

	firstPage, err := svc.ListMyTextures(ctx, owner.ID, "", 1, "skin")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	cursor, _ := firstPage["next_cursor"].(string)
	if len(firstItems) != 1 || cursor == "" {
		t.Fatalf("ListMyTextures first page should include one item and next cursor: %#v", firstPage)
	}
	secondPage, err := svc.ListMyTextures(ctx, owner.ID, cursor, 10, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if secondItems := secondPage["items"].([]map[string]any); len(secondItems) != 1 || secondItems[0]["hash"] == firstItems[0]["hash"] {
		t.Fatalf("ListMyTextures cursor should advance to next item: first=%#v second=%#v", firstPage, secondPage)
	}

	public, err := svc.PublicLibrary(ctx, "", 1, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := public["items"].([]map[string]any)
	publicCursor, _ := public["next_cursor"].(string)
	if len(publicItems) != 1 || publicCursor == "" || publicItems[0]["usage_count"] != int64(2) {
		t.Fatalf("most_used public library first page mismatch: %#v", public)
	}
	nextPublic, err := svc.PublicLibrary(ctx, publicCursor, 10, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	nextItems := nextPublic["items"].([]map[string]any)
	if len(nextItems) != 1 || nextItems[0]["hash"] == publicItems[0]["hash"] {
		t.Fatalf("most_used public library cursor should advance exactly: first=%#v next=%#v", public, nextPublic)
	}
}

func TestDeleteUserRecountsSharedLibraryButDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-profile-delete-owner@test.com", "Password123", "ProfileDeleteOwner", false)
	target := testutil.CreateUser(t, db, "site-profile-delete-target@test.com", "Password123", "ProfileDeleteTarget", false)
	other := testutil.CreateUser(t, db, "site-profile-delete-other@test.com", "Password123", "ProfileDeleteOther", false)

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, target.ID, "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.DeleteUser(ctx, target.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteUser returned ok=%v err=%v", ok, err)
	}
	assertServicePublicUsage(t, svc, "delete_user_shared_skin", int64(2))
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}

func assertServicePublicUsage(t *testing.T, svc anyPublicLibrary, hash string, want int64) {
	t.Helper()
	page, err := svc.PublicLibrary(context.Background(), "", 10, "skin", "", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page["items"].([]map[string]any) {
		if item["hash"] == hash {
			if item["usage_count"] != want {
				t.Fatalf("usage_count mismatch for %s want=%d got=%#v", hash, want, item)
			}
			return
		}
	}
	t.Fatalf("missing public library item %s in %#v", hash, page)
}

type anyPublicLibrary interface {
	PublicLibrary(context.Context, string, int, string, string, string) (map[string]any, error)
}
