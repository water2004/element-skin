package profile_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}

	skin := "profile_service_skin"
	cape := "profile_service_cape"
	if err := db.Profiles.UpdateSkin(ctx, created["id"].(string), &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(ctx, created["id"].(string), &cape); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearProfileTexture(ctx, testUserActor(user.ID), created["id"].(string), "skin"); err != nil {
		t.Fatal(err)
	}
	cleared, err := db.Profiles.GetByID(ctx, created["id"].(string))
	if err != nil || cleared == nil || cleared.SkinHash != nil || cleared.CapeHash == nil || *cleared.CapeHash != cape {
		t.Fatalf("ClearProfileTexture should clear only skin: profile=%#v err=%v", cleared, err)
	}

	deletable := testutil.CreateProfile(t, db, user.ID, "site_profile_owned_delete", "OwnedDelete")
	if err := svc.DeleteProfile(ctx, testUserActor(user.ID), deletable.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, deletable.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfile should remove owned profile exactly: profile=%#v err=%v", got, err)
	}
}

func TestProfilesRejectInvalidProfileInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
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
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "", "default")
			return err
		}, 400, "profile_name.validate.required"},
		{"invalid create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "bad-name!", "default")
			return err
		}, 400, "profile_name.validate.invalid"},
		{"duplicate create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), existing.Name, "default")
			return err
		}, 400, "profile_name.reserve.conflict"},
		{"empty update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "")
		}, 400, "profile_name.validate.required"},
		{"invalid update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "bad-name!")
		}, 400, "profile_name.validate.invalid"},
		{"duplicate update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, existing.Name)
		}, 400, "profile_name.reserve.conflict"},
		{"foreign update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), foreign.ID, "StolenProfileSvc")
		}, 403, "permission.check.denied"},
		{"missing update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), "missing-profile", "MissingProfileSvc")
		}, 404, "profile.resolve.not_found"},
		{"foreign delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), foreign.ID)
		}, 403, "permission.check.denied"},
		{"missing delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), "missing-profile")
		}, 404, "profile.resolve.not_found"},
		{"invalid clear texture type", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), target.ID, "elytra")
		}, 400, "texture_type.validate.invalid"},
		{"foreign clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), foreign.ID, "skin")
		}, 403, "permission.check.denied"},
		{"missing clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), "missing-profile", "skin")
		}, 404, "profile.resolve.not_found"},
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

func TestCreateProfileMapsDatabaseIDConflictExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-id-conflict@test.com", "Password123", "ProfileIDConflict", false)
	existing := testutil.CreateProfile(t, db, user.ID, "forced_profile_id_conflict", "ExistingID")
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION force_profile_id_conflict() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'ForcedUUID' THEN
				NEW.id := 'forced_profile_id_conflict';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER force_profile_id_conflict
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION force_profile_id_conflict();
	`); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ForcedUUID", "slim")
	if result != nil || !httpError(err, 400, "profile_uuid.reserve.conflict") {
		t.Fatalf("forced profile ID conflict result=%#v err=%#v; want nil and exact 400", result, err)
	}
	if stored, err := db.Profiles.GetByName(ctx, "ForcedUUID"); err != nil || stored != nil {
		t.Fatalf("forced UUID conflict persisted target name: profile=%#v err=%v", stored, err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, existing.ID)
	if err != nil || unchanged == nil || unchanged.Name != "ExistingID" || unchanged.TextureModel != "default" {
		t.Fatalf("forced UUID conflict changed existing profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestCreateProfileOfflineModeUsesDeterministicIDAndRejectsIDConflict(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-offline-mode@test.com", "Password123", "ProfileOfflineMode", false)
	other := testutil.CreateUser(t, db, "profile-offline-conflict@test.com", "Password123", "ProfileOfflineConflict", false)
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := svc.CreateProfile(ctx, testUserActor(user.ID), "OfflineProfile", "wide")
	if err != nil {
		t.Fatal(err)
	}
	if created["id"] != util.OfflineUUIDNoDash("OfflineProfile") || created["model"] != "default" {
		t.Fatalf("offline profile creation mismatch: %#v", created)
	}
	conflictingID := util.OfflineUUIDNoDash("OfflineClash")
	testutil.CreateProfile(t, db, other.ID, conflictingID, "DifferentName")
	result, err := svc.CreateProfile(ctx, testUserActor(user.ID), "OfflineClash", "slim")
	if result != nil || !httpError(err, 400, "profile_uuid.reserve.conflict") {
		t.Fatalf("offline profile ID conflict result=%#v err=%#v", result, err)
	}
	if stored, err := db.Profiles.GetByName(ctx, "OfflineClash"); err != nil || stored != nil {
		t.Fatalf("offline ID conflict must not create requested name: profile=%#v err=%v", stored, err)
	}
}
