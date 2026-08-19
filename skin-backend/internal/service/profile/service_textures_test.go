package profile_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSetProfileTextureSkipsExactNoOpWrites(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-noop@test.com", "Password123", "ProfileNoop", false)
	profile := testutil.CreateProfile(t, db, user.ID, "profile_noop_values", "ProfileNoopValues")
	empty := testutil.CreateProfile(t, db, user.ID, "profile_noop_empty", "ProfileNoopEmpty")
	adminActor := testActorWithCodes("profile-texture-admin", "profile.update.any")
	skin := "same_skin_hash"
	cape := "same_cape_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatalf("seed skin: %v", err)
	}
	if err := db.Profiles.UpdateCape(ctx, profile.ID, &cape); err != nil {
		t.Fatalf("seed cape: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_profile_noop_updates() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'profile update should not run' USING ERRCODE='23514';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_profile_noop_updates
		BEFORE UPDATE ON profiles
		FOR EACH ROW
		WHEN (OLD.id IN ('profile_noop_values', 'profile_noop_empty'))
		EXECUTE FUNCTION reject_profile_noop_updates();
	`); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		profileID   string
		textureType string
		hash        *string
	}{
		{name: "same skin", profileID: profile.ID, textureType: "skin", hash: &skin},
		{name: "same cape", profileID: profile.ID, textureType: "cape", hash: &cape},
		{name: "already clear skin", profileID: empty.ID, textureType: "skin", hash: nil},
		{name: "already clear cape", profileID: empty.ID, textureType: "cape", hash: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.SetProfileTexture(ctx, adminActor, tc.profileID, tc.textureType, tc.hash); err != nil {
				t.Fatalf("exact no-op should skip database update: %v", err)
			}
		})
	}

	different := "different_skin_hash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, different, "skin", "different skin", false, "default"); err != nil {
		t.Fatal(err)
	}
	err := svc.SetProfileTexture(ctx, adminActor, profile.ID, "skin", &different)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("changed value should hit rejection trigger exactly, got %#v", err)
	}
}

func TestSetProfileTextureCapeAndMissingProfileExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-set-cape@test.com", "Password123", "ProfileSetCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "profile_set_cape", "ProfileSetCape")
	actor := testActorWithCodes("profile-set-cape-admin", "profile.update.any")
	cape := "new_cape_hash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, cape, "cape", "new cape", false, "default"); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetProfileTexture(ctx, actor, profile.ID, "cape", &cape); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.CapeHash == nil || *updated.CapeHash != cape || updated.SkinHash != nil {
		t.Fatalf("SetProfileTexture cape mismatch: profile=%#v err=%v", updated, err)
	}
	if err := svc.SetProfileTexture(ctx, actor, "missing-set-cape", "skin", nil); !httpError(err, 404, "profile.resolve.not_found") {
		t.Fatalf("SetProfileTexture missing profile mismatch: %#v", err)
	}
}
