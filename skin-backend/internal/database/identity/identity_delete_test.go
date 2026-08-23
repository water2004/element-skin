package identity_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDeleteIdentityDetachesOfficialBindingsAndPreservesProfilesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "identity-delete-store@test.com", "Password123", "IdentityDeleteStore", false)
	provider := providerDeleteTestProvider("identity-delete-provider", "https://identity-delete.example")
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	for _, item := range []model.ExternalIdentity{
		{ID: "identity-delete-target", UserID: user.ID, ProviderID: provider.ID, Subject: "target", CreatedAt: 1, UpdatedAt: 1},
		{ID: "identity-delete-other", UserID: user.ID, ProviderID: provider.ID, Subject: "other", CreatedAt: 2, UpdatedAt: 2},
	} {
		if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{IdentityID: item.ID, UpdatedAt: item.UpdatedAt}); err != nil {
			t.Fatal(err)
		}
	}
	targetProfile := testutil.CreateProfile(t, db, user.ID, "identity_delete_target_profile", "IdentityDeleteTarget")
	otherProfile := testutil.CreateProfile(t, db, user.ID, "identity_delete_other_profile", "IdentityDeleteOther")
	for _, binding := range []struct {
		id, identityID, profileID, remoteUUID string
	}{
		{"identity-delete-target-binding", "identity-delete-target", targetProfile.ID, "identity-delete-target-remote"},
		{"identity-delete-other-binding", "identity-delete-other", otherProfile.ID, "identity-delete-other-remote"},
	} {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO official_profile_bindings
				(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
			VALUES ($1,$2,$3,$4,'Remote',3,3)
		`, binding.id, binding.identityID, binding.profileID, binding.remoteUUID); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := db.Identities.DeleteIdentity(ctx, "identity-delete-target", user.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteIdentity deleted=%v err=%v", deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, "identity-delete-target", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, "identity-delete-target", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "identity-delete-target-binding", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM profiles WHERE id=$1`, targetProfile.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, "identity-delete-other", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "identity-delete-other-binding", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM profiles WHERE id=$1`, otherProfile.ID, 1)

	deleted, err = db.Identities.DeleteIdentity(ctx, "identity-delete-other", "another-user")
	if err != nil || deleted {
		t.Fatalf("cross-account DeleteIdentity deleted=%v err=%v", deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, "identity-delete-other", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "identity-delete-other-binding", 1)
}

func TestDeleteIdentityRollsBackBindingDetachWhenIdentityDeletionFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "identity-delete-rollback@test.com", "Password123", "IdentityDeleteRollback", false)
	provider := providerDeleteTestProvider("identity-delete-rollback-provider", "https://identity-delete-rollback.example")
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	item := model.ExternalIdentity{ID: "identity-delete-rollback", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{IdentityID: item.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "identity_delete_rollback_profile", "IdentityDeleteRollback")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('identity-delete-rollback-binding',$1,$2,'identity-delete-rollback-remote','Remote',2,2)
	`, item.ID, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_identity_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'reject identity delete';
		END
		$$;
		CREATE TRIGGER reject_identity_delete
		BEFORE DELETE ON external_identities
		FOR EACH ROW EXECUTE FUNCTION reject_identity_delete();
	`); err != nil {
		t.Fatal(err)
	}

	deleted, err := db.Identities.DeleteIdentity(ctx, item.ID, user.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || deleted {
		t.Fatalf("failed DeleteIdentity deleted=%v err=%#v", deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "identity-delete-rollback-binding", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM profiles WHERE id=$1`, profile.ID, 1)
}

func TestDeleteIdentityPreservesEverythingWhenBindingDetachFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "identity-detach-failure@test.com", "Password123", "IdentityDetachFailure", false)
	provider := providerDeleteTestProvider("identity-detach-failure-provider", "https://identity-detach-failure.example")
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	item := model.ExternalIdentity{ID: "identity-detach-failure", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{IdentityID: item.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "identity_detach_failure_profile", "IdentityDetachFailure")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('identity-detach-failure-binding',$1,$2,'identity-detach-failure-remote','Remote',2,2)
	`, item.ID, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_binding_detach() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'reject binding detach';
		END
		$$;
		CREATE TRIGGER reject_binding_detach
		BEFORE DELETE ON official_profile_bindings
		FOR EACH ROW EXECUTE FUNCTION reject_binding_detach();
	`); err != nil {
		t.Fatal(err)
	}

	deleted, err := db.Identities.DeleteIdentity(ctx, item.ID, user.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || deleted {
		t.Fatalf("failed binding detach deleted=%v err=%#v", deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "identity-detach-failure-binding", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM profiles WHERE id=$1`, profile.ID, 1)
}

func TestIdentityAuthorizationWritesAreAtomicAcrossExactFailures(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "identity-write-atomic@test.com", "Password123", "IdentityWriteAtomic", false)
	provider := providerDeleteTestProvider("identity-write-atomic-provider", "https://identity-write-atomic.example")
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	item := model.ExternalIdentity{
		ID: "identity-write-atomic", UserID: user.ID, ProviderID: provider.ID, Subject: "subject",
		Email: "before@example.com", DisplayName: "Before", CreatedAt: 1, UpdatedAt: 1,
	}
	credential := model.ExternalIdentityCredential{
		IdentityID: item.ID, RefreshTokenCiphertext: "before-token", GrantedScopes: []string{"openid"}, UpdatedAt: 1,
	}
	if err := db.Identities.CreateIdentity(ctx, item, credential); err != nil {
		t.Fatal(err)
	}
	beforeIdentity, err := db.Identities.GetIdentity(ctx, item.ID)
	if err != nil || beforeIdentity == nil {
		t.Fatalf("before identity=%#v err=%v", beforeIdentity, err)
	}
	beforeCredential, err := db.Identities.GetCredential(ctx, item.ID)
	if err != nil || beforeCredential == nil {
		t.Fatalf("before credential=%#v err=%v", beforeCredential, err)
	}

	duplicate := item
	duplicate.Subject = "changed-subject"
	err = db.Identities.CreateIdentity(ctx, duplicate, credential)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate identity error=%#v", err)
	}

	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_identity_credential_write() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'reject identity credential write';
		END
		$$;
		CREATE TRIGGER reject_identity_credential_write
		BEFORE INSERT OR UPDATE ON external_identity_credentials
		FOR EACH ROW EXECUTE FUNCTION reject_identity_credential_write();
	`); err != nil {
		t.Fatal(err)
	}
	failedCreate := item
	failedCreate.ID = "identity-write-atomic-failed-create"
	failedCreate.Subject = "failed-create"
	err = db.Identities.CreateIdentity(ctx, failedCreate, model.ExternalIdentityCredential{IdentityID: failedCreate.ID, UpdatedAt: 2})
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
		t.Fatalf("credential-failed create error=%#v", err)
	}
	if stored, getErr := db.Identities.GetIdentity(ctx, failedCreate.ID); getErr != nil || stored != nil {
		t.Fatalf("credential-failed create identity=%#v err=%v", stored, getErr)
	}

	updatedIdentity := *beforeIdentity
	updatedIdentity.Email = "after@example.com"
	updatedIdentity.DisplayName = "After"
	updatedIdentity.UpdatedAt = 3
	updatedCredential := *beforeCredential
	updatedCredential.RefreshTokenCiphertext = "after-token"
	updatedCredential.GrantedScopes = []string{"openid", "email"}
	updatedCredential.UpdatedAt = 3
	updated, err := db.Identities.UpdateIdentityAuthorization(ctx, updatedIdentity, updatedCredential)
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || updated {
		t.Fatalf("credential-failed update updated=%v err=%#v", updated, err)
	}
	afterIdentity, identityErr := db.Identities.GetIdentity(ctx, item.ID)
	afterCredential, credentialErr := db.Identities.GetCredential(ctx, item.ID)
	if identityErr != nil || credentialErr != nil || !reflect.DeepEqual(afterIdentity, beforeIdentity) || !reflect.DeepEqual(afterCredential, beforeCredential) {
		t.Fatalf("failed authorization update mutated state: identity=%#v/%v credential=%#v/%v", afterIdentity, identityErr, afterCredential, credentialErr)
	}
}
