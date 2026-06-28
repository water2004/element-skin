package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

// TestAccountPermissionDenials verifies that account service methods reject actors
// who lack the required self-scoped permissions.
func TestAccountPermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-account-deny@test.com", "Password123", "PermAccountDeny", false)

	for _, tc := range []struct {
		name      string
		actorCode string
		call      func(permission.Actor) error
		status    int
		detail    string
	}{
		{
			name:      "UpdateMe without account.update.self",
			actorCode: "account.read.self",
			call: func(a permission.Actor) error {
				return svc.UpdateMe(ctx, a, map[string]any{"preferred_language": "en_US"})
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "ChangePassword without account_password.update.self",
			actorCode: "account.update.self",
			call: func(a permission.Actor) error {
				return svc.ChangePassword(ctx, a, "Password123", "NewPassword123")
			},
			status: 403,
			detail: "permission denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := testActorWithCodes(user.ID, tc.actorCode)
			err := tc.call(actor)
			if !httpError(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

// TestProfilePermissionDenials verifies that profile service methods reject actors
// who lack the required permissions.
func TestProfilePermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
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
			detail: "permission denied",
		},
		{
			name:      "ListMyProfiles without profile.read.owned",
			actorCode: "profile.create.owned",
			call: func(a permission.Actor) error {
				_, err := svc.ListMyProfiles(ctx, a, "", 10)
				return err
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "UpdateProfile without profile.update.owned",
			actorCode: "profile.read.owned",
			call: func(a permission.Actor) error {
				return svc.UpdateProfile(ctx, a, profile.ID, "RenamedPerm")
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "DeleteProfile without profile.delete.owned",
			actorCode: "profile.update.owned",
			call: func(a permission.Actor) error {
				return svc.DeleteProfile(ctx, a, profile.ID)
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "DeleteProfileByID without profile.delete.any",
			actorCode: "profile.read.any",
			call: func(a permission.Actor) error {
				return svc.DeleteProfileByID(ctx, a, profile.ID)
			},
			status: 403,
			detail: "permission denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := testActorWithCodes(user.ID, tc.actorCode)
			err := tc.call(actor)
			if !httpError(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

// TestTexturePermissionDenials verifies that texture service methods reject actors
// who lack the required permissions.
func TestTexturePermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
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
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "ListMyTextures without texture.read.owned",
			actorCode: "texture.create.owned",
			call: func(a permission.Actor) error {
				_, err := svc.ListMyTextures(ctx, a, "", 10, "skin")
				return err
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "DeleteTexture without texture.delete.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.DeleteTexture(ctx, a, "perm_texture_skin", "skin")
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "AddTextureToWardrobe without wardrobe_entry.add.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.AddTextureToWardrobe(ctx, a, "perm_texture_skin", "skin")
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "ApplyTextureToProfile without texture.apply.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				return svc.ApplyTextureToProfile(ctx, a, profile.ID, "perm_texture_skin", "skin")
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "UpdateTexture note without texture.update_metadata.owned",
			actorCode: "texture.read.owned",
			call: func(a permission.Actor) error {
				_, err := svc.UpdateTexture(ctx, a, "perm_texture_skin", "skin", map[string]any{"note": "new"})
				return err
			},
			status: 403,
			detail: "permission denied",
		},
		{
			name:      "UpdateTexture is_public without texture.update_visibility.owned",
			actorCode: "texture.update_metadata.owned",
			call: func(a permission.Actor) error {
				_, err := svc.UpdateTexture(ctx, a, "perm_texture_skin", "skin", map[string]any{"is_public": true})
				return err
			},
			status: 403,
			detail: "permission denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := testActorWithCodes(user.ID, tc.actorCode)
			err := tc.call(actor)
			if !httpError(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

// TestDeleteUserPermissionDenials verifies that DeleteUser correctly distinguishes
// self-delete from admin-delete permissions.
func TestDeleteUserPermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-delete-user@test.com", "Password123", "PermDeleteUser", false)
	other := testutil.CreateUser(t, db, "perm-delete-other@test.com", "Password123", "PermDeleteOther", false)

	for _, tc := range []struct {
		name      string
		actorCode string
		targetID  string
		status    int
		detail    string
	}{
		{
			name:      "Delete self without account.delete.self",
			actorCode: "account.read.self",
			targetID:  user.ID,
			status:    403,
			detail:    "permission denied",
		},
		{
			name:      "Delete other without account.delete.any",
			actorCode: "account.delete.self",
			targetID:  other.ID,
			status:    403,
			detail:    "permission denied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := testActorWithCodes(user.ID, tc.actorCode)
			_, err := svc.DeleteUser(ctx, actor, tc.targetID)
			if !httpError(err, tc.status, tc.detail) {
				t.Fatalf("expected %d %q, got %#v", tc.status, tc.detail, err)
			}
		})
	}
}

// TestSetProfileTexturePermissionDenial verifies that SetProfileTexture requires
// the admin-scoped profile.update.any permission.
func TestSetProfileTexturePermissionDenial(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-set-texture@test.com", "Password123", "PermSetTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_set_texture", "PermSetTexture")
	hash := "some_hash"

	actor := testActorWithCodes(user.ID, "profile.update.owned")
	err := svc.SetProfileTexture(ctx, actor, profile.ID, "skin", &hash)
	if !httpError(err, 403, "permission denied") {
		t.Fatalf("SetProfileTexture with owned scope should be denied, got %#v", err)
	}
}

// TestClearProfileTexturePermissionDenial verifies that ClearProfileTexture
// rejects actors without either owned or bound_profile permissions.
func TestClearProfileTexturePermissionDenial(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-clear-texture@test.com", "Password123", "PermClearTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_clear_texture", "PermClearTexture")

	actor := testActorWithCodes(user.ID, "texture.read.owned")
	err := svc.ClearProfileTexture(ctx, actor, profile.ID, "skin")
	if !httpError(err, 403, "permission denied") {
		t.Fatalf("ClearProfileTexture without clear permission should be denied, got %#v", err)
	}
}

// TestClearProfileTextureWithBoundActor verifies that an actor with
// texture.clear.bound_profile can clear the texture when BoundProfileID matches.
func TestClearProfileTextureWithBoundActor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
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

// TestApplyTextureWithBoundActor verifies that an actor with
// texture.apply.bound_profile can apply textures when BoundProfileID matches.
func TestApplyTextureWithBoundActor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-apply-bound@test.com", "Password123", "PermApplyBound", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_apply_bound", "PermApplyBound")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "bound_apply_skin", "skin", "Bound Apply", false, "default"); err != nil {
		t.Fatal(err)
	}

	actor := testActorWithCodes(user.ID, "texture.apply.bound_profile")
	actor.BoundProfileID = profile.ID
	if err := svc.ApplyTextureToProfile(ctx, actor, profile.ID, "bound_apply_skin", "skin"); err != nil {
		t.Fatalf("ApplyTextureToProfile with bound_profile scope should succeed, got %#v", err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != "bound_apply_skin" {
		t.Fatalf("skin should be applied: profile=%#v err=%v", updated, err)
	}
}

// TestApplyTextureBoundMismatch verifies that a bound_profile actor cannot
// apply textures to a profile that doesn't match their BoundProfileID.
func TestApplyTextureBoundMismatch(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "perm-bound-mismatch@test.com", "Password123", "PermBoundMismatch", false)
	profile1 := testutil.CreateProfile(t, db, owner.ID, "perm_bound_mismatch_1", "BoundMismatch1")
	profile2 := testutil.CreateProfile(t, db, owner.ID, "perm_bound_mismatch_2", "BoundMismatch2")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "bound_mismatch_skin", "skin", "Mismatch", false, "default"); err != nil {
		t.Fatal(err)
	}

	actor := testActorWithCodes(owner.ID, "texture.apply.bound_profile")
	actor.BoundProfileID = profile1.ID
	err := svc.ApplyTextureToProfile(ctx, actor, profile2.ID, "bound_mismatch_skin", "skin")
	if !httpError(err, 403, "permission denied") {
		t.Fatalf("bound actor applying to wrong profile should be denied, got %#v", err)
	}
}

// TestPermissionDenialDoesNotMutateState verifies that failed permission checks
// leave database state unchanged.
func TestPermissionDenialDoesNotMutateState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "perm-no-mutate@test.com", "Password123", "PermNoMutate", false)
	profile := testutil.CreateProfile(t, db, user.ID, "perm_no_mutate", "PermNoMutate")
	actor := testActorWithCodes(user.ID, "profile.read.owned")

	err := svc.UpdateProfile(ctx, actor, profile.ID, "StolenName")
	if !httpError(err, 403, "permission denied") {
		t.Fatalf("expected permission denied, got %#v", err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.Name != "PermNoMutate" {
		t.Fatalf("profile name should be unchanged after denied update: profile=%#v err=%v", unchanged, err)
	}
}
