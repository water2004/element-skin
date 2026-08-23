package profile_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestProfileServiceAdminListProfilesExactPages(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	admin := testutil.CreateUser(t, db, "admin-profile-list@test.com", "Password123", "AdminProfileList", true)
	owner := testutil.CreateUser(t, db, "owner-profile-list@test.com", "Password123", "OwnerProfileList", false)
	other := testutil.CreateUser(t, db, "other-profile-list@test.com", "Password123", "OtherProfileList", false)
	ownerProfile := testutil.CreateProfile(t, db, owner.ID, "admin_list_owner_profile", "AdminListOwner")
	otherProfile := testutil.CreateProfile(t, db, other.ID, "admin_list_other_profile", "AdminListOther")
	actor := testActorWithCodes(admin.ID, "profile.read.any")

	allPage, err := svc.ListAllProfiles(ctx, actor, "", 10, "owner-profile-list")
	if err != nil {
		t.Fatal(err)
	}
	allItems := allPage["items"].([]map[string]any)
	if allPage["page_size"] != 1 || allPage["has_next"] != false || allPage["next_cursor"] != "" || len(allItems) != 1 {
		t.Fatalf("ListAllProfiles page mismatch: %#v", allPage)
	}
	if allItems[0]["id"] != ownerProfile.ID || allItems[0]["name"] != ownerProfile.Name ||
		allItems[0]["user_id"] != owner.ID || allItems[0]["owner_email"] != owner.Email ||
		allItems[0]["owner_display_name"] != owner.DisplayName {
		t.Fatalf("ListAllProfiles item mismatch: %#v", allItems[0])
	}

	userPage, err := svc.ListProfilesByUser(ctx, actor, other.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	userItems := userPage["items"].([]map[string]any)
	if userPage["page_size"] != 1 || userPage["has_next"] != false || userPage["next_cursor"] != "" || len(userItems) != 1 ||
		userItems[0]["id"] != otherProfile.ID || userItems[0]["name"] != otherProfile.Name {
		t.Fatalf("ListProfilesByUser page mismatch: %#v", userPage)
	}
}

func TestProfileServiceAdminListProfilesRejectsAccessAndBadCursorExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	admin := testutil.CreateUser(t, db, "admin-profile-list-invalid@test.com", "Password123", "AdminProfileListInvalid", true)
	actor := testActorWithCodes(admin.ID, "profile.read.any")

	if _, err := svc.ListAllProfiles(ctx, permission.Actor{}, "", 10, ""); !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ListAllProfiles without permission mismatch: %#v", err)
	}
	if _, err := svc.ListAllProfiles(ctx, actor, "bad-cursor", 10, ""); !httpError(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListAllProfiles bad cursor mismatch: %#v", err)
	}
	if _, err := svc.ListProfilesByUser(ctx, permission.Actor{}, admin.ID, "", 10); !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ListProfilesByUser without permission mismatch: %#v", err)
	}
	if _, err := svc.ListProfilesByUser(ctx, actor, admin.ID, "bad-cursor", 10); !httpError(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListProfilesByUser bad cursor mismatch: %#v", err)
	}
}

func TestProfileServiceUpdateAnyProfileExactStateAndErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	admin := testutil.CreateUser(t, db, "admin-profile-update-any@test.com", "Password123", "AdminProfileUpdateAny", true)
	owner := testutil.CreateUser(t, db, "owner-profile-update-any@test.com", "Password123", "OwnerProfileUpdateAny", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "admin_update_any_profile", "AnyOld")
	conflict := testutil.CreateProfile(t, db, owner.ID, "admin_update_any_conflict", "AnyConflict")
	actor := testActorWithCodes(admin.ID, "profile.update.any")

	if err := svc.UpdateAnyProfile(ctx, actor, profile.ID, "AnyNew"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.Name != "AnyNew" {
		t.Fatalf("UpdateAnyProfile should persist exact name: profile=%#v err=%v", updated, err)
	}
	if err := svc.UpdateAnyProfile(ctx, actor, profile.ID, ""); err != nil {
		t.Fatal(err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.Name != "AnyNew" {
		t.Fatalf("empty UpdateAnyProfile should leave name unchanged: profile=%#v err=%v", unchanged, err)
	}

	if err := svc.UpdateAnyProfile(ctx, permission.Actor{}, profile.ID, "NoPermission"); !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UpdateAnyProfile without permission mismatch: %#v", err)
	}
	if err := svc.UpdateAnyProfile(ctx, actor, "missing-admin-update-any", "MissingName"); !httpError(err, http.StatusNotFound, "profile.resolve.not_found") {
		t.Fatalf("UpdateAnyProfile missing profile mismatch: %#v", err)
	}
	if err := svc.UpdateAnyProfile(ctx, actor, profile.ID, "bad-name!"); !httpError(err, http.StatusBadRequest, "profile_name.validate.invalid") {
		t.Fatalf("UpdateAnyProfile invalid name mismatch: %#v", err)
	}
	if err := svc.UpdateAnyProfile(ctx, actor, profile.ID, conflict.Name); !httpError(err, http.StatusConflict, "profile_name.reserve.conflict") {
		t.Fatalf("UpdateAnyProfile conflict mismatch: %#v", err)
	}
}
