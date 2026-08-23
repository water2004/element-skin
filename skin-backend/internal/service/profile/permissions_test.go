package profile_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/permission"
	profilesvc "element-skin/backend/internal/service/profile"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestProfilePermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := profilesvc.Service{DB: db}
	user := testutil.CreateUser(t, db, "perm-profile-deny@test.com", "Password123", "PermProfileDeny", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_profile_deny", "PermProfileDeny")

	for _, tc := range []struct {
		name      string
		actorCode string
		call      func(permission.Actor) error
		status    int
		detail    string
	}{
		{
			name:      "CreateProfile without profile.create.owned",
			actorCode: "profile.read.owned",
			call: func(a permission.Actor) error {
				_, err := svc.CreateProfile(ctx, a, "NewPermProfile", "default")
				return err
			},
			status: 403,
			detail: "permission.check.denied",
		},
		{
			name:      "ListMyProfiles without profile.read.owned",
			actorCode: "profile.create.owned",
			call: func(a permission.Actor) error {
				_, err := svc.ListMyProfiles(ctx, a, "", 10)
				return err
			},
			status: 403,
			detail: "permission.check.denied",
		},
		{
			name:      "UpdateProfile without profile.update.owned",
			actorCode: "profile.read.owned",
			call: func(a permission.Actor) error {
				return svc.UpdateProfile(ctx, a, profile.ID, "RenamedPerm")
			},
			status: 403,
			detail: "permission.check.denied",
		},
		{
			name:      "DeleteProfile without profile.delete.owned",
			actorCode: "profile.update.owned",
			call: func(a permission.Actor) error {
				return svc.DeleteProfile(ctx, a, profile.ID)
			},
			status: 403,
			detail: "permission.check.denied",
		},
		{
			name:      "DeleteProfileByID without profile.delete.any",
			actorCode: "profile.read.any",
			call: func(a permission.Actor) error {
				return svc.DeleteProfileByID(ctx, a, profile.ID)
			},
			status: 403,
			detail: "permission.check.denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(testActorWithCodes(user.ID, tc.actorCode))
			if !httpError(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

func TestSetProfileTexturePermissionDenial(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := profilesvc.Service{DB: db}
	user := testutil.CreateUser(t, db, "perm-set-texture@test.com", "Password123", "PermSetTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_set_texture", "PermSetTexture")
	hash := "some_hash"

	actor := testActorWithCodes(user.ID, "profile.update.owned")
	err := svc.SetProfileTexture(ctx, actor, profile.ID, "skin", &hash)
	if !httpError(err, 403, "permission.check.denied") {
		t.Fatalf("SetProfileTexture with owned scope should be denied, got %#v", err)
	}
}

func TestClearProfileTexturePermissionDenial(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := profilesvc.Service{DB: db}
	user := testutil.CreateUser(t, db, "perm-clear-texture@test.com", "Password123", "PermClearTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_clear_texture", "PermClearTexture")

	actor := testActorWithCodes(user.ID, "texture.read.owned")
	err := svc.ClearProfileTexture(ctx, actor, profile.ID, "skin")
	if !httpError(err, 403, "permission.check.denied") {
		t.Fatalf("ClearProfileTexture without clear permission should be denied, got %#v", err)
	}
}

func TestClearProfileTextureWithBoundActor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := profilesvc.Service{DB: db}
	user := testutil.CreateUser(t, db, "perm-clear-bound@test.com", "Password123", "PermClearBound", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_clear_bound", "PermClearBound")
	skin := "bound_skin_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}

	actor := testActorWithCodes(user.ID, "texture.clear.bound_profile")
	actor.BoundProfileID = profile.ID
	if err := svc.ClearProfileTexture(ctx, actor, profile.ID, "skin"); err != nil {
		t.Fatalf("ClearProfileTexture with bound_profile scope should succeed, got %#v", err)
	}
	cleared, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || cleared == nil || cleared.SkinHash != nil {
		t.Fatalf("skin should be cleared: profile=%#v err=%v", cleared, err)
	}
}

func TestProfilePermissionDenialDoesNotMutateState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := profilesvc.Service{DB: db}
	user := testutil.CreateUser(t, db, "perm-no-mutate@test.com", "Password123", "PermNoMutate", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_no_mutate", "PermNoMutate")
	actor := testActorWithCodes(user.ID, "profile.read.owned")

	err := svc.UpdateProfile(ctx, actor, profile.ID, "StolenName")
	if !httpError(err, 403, "permission.check.denied") {
		t.Fatalf("expected permission denied, got %#v", err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.Name != "PermNoMutate" {
		t.Fatalf("profile name should be unchanged after denied update: profile=%#v err=%v", unchanged, err)
	}
}

func httpError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}
