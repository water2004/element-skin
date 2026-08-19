package profile_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestProfilesCursorsAndAdminDeleteByID(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_profile_admin_delete", "AdminDeleteProfileSvc")

	if _, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "not-base64", 10); !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid profile cursor should reject exactly, got %#v", err)
	}

	adminActor := testActorWithCodes("admin-delete-profile", "profile.delete.any")
	if err := svc.DeleteProfileByID(ctx, adminActor, profile.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfileByID should remove profile regardless of owner: profile=%#v err=%v", got, err)
	}
	if err := svc.DeleteProfileByID(ctx, adminActor, profile.ID); !httpError(err, 404, "profile.resolve.not_found") {
		t.Fatalf("DeleteProfileByID missing profile should reject exactly, got %#v", err)
	}
}

func TestProfileServiceAdminListsRejectIncompleteCursorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testActorWithCodes("admin-profile-incomplete-cursor", "profile.read.any")
	cursor := util.EncodeCursor(map[string]any{"unexpected": "value"})

	if result, err := svc.ListAllProfiles(ctx, actor, cursor, 10, ""); result != nil || !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListAllProfiles incomplete cursor result=%#v err=%#v", result, err)
	}
	if result, err := svc.ListProfilesByUser(ctx, actor, "any-user", cursor, 10); result != nil || !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListProfilesByUser incomplete cursor result=%#v err=%#v", result, err)
	}
}

func TestPrivateProfileListRejectsIncompleteCursor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "private-cursor@test.com", "Password123", "PrivateCursor", false)

	profileResult, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), util.EncodeCursor(map[string]any{
		"unexpected": "value",
	}), 10)
	if profileResult != nil || !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListMyProfiles result=%#v err=%#v; want nil and exact invalid cursor", profileResult, err)
	}
}
