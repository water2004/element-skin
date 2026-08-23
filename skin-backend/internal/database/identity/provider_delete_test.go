package identity_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDeleteProviderRemovesAllIdentityRecordsAndPreservesUnrelatedRowsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "provider-delete-store@test.com", "Password123", "ProviderDeleteStore", false)
	target := providerDeleteTestProvider("provider-delete-target", "https://target.identity.example")
	unrelated := providerDeleteTestProvider("provider-delete-unrelated", "https://unrelated.identity.example")
	for _, provider := range []model.IdentityProvider{target, unrelated} {
		if err := db.Identities.CreateProvider(ctx, provider); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []model.ExternalIdentity{
		{ID: "provider-delete-identity-b", UserID: user.ID, ProviderID: target.ID, Subject: "subject-b", CreatedAt: 2, UpdatedAt: 2},
		{ID: "provider-delete-identity-a", UserID: user.ID, ProviderID: target.ID, Subject: "subject-a", CreatedAt: 1, UpdatedAt: 1},
		{ID: "provider-delete-identity-unrelated", UserID: user.ID, ProviderID: unrelated.ID, Subject: "subject-unrelated", CreatedAt: 3, UpdatedAt: 3},
	} {
		if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{
			IdentityID: item.ID, RefreshTokenCiphertext: "ciphertext-" + item.ID, UpdatedAt: item.UpdatedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	targetProfile := testutil.CreateProfile(t, db, user.ID, "provider_delete_profile_target", "ProviderDeleteTarget")
	unrelatedProfile := testutil.CreateProfile(t, db, user.ID, "provider_delete_profile_other", "ProviderDeleteOther")
	for _, values := range []struct {
		id, identityID, profileID, remoteUUID string
	}{
		{"provider-delete-binding-target", "provider-delete-identity-a", targetProfile.ID, "provider-delete-remote-target"},
		{"provider-delete-binding-unrelated", "provider-delete-identity-unrelated", unrelatedProfile.ID, "provider-delete-remote-unrelated"},
	} {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO official_profile_bindings
				(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
			VALUES ($1,$2,$3,$4,'Remote',4,4)
		`, values.id, values.identityID, values.profileID, values.remoteUUID); err != nil {
			t.Fatal(err)
		}
	}

	identityIDs, deleted, err := db.Identities.DeleteProvider(ctx, target.ID)
	if err != nil || !deleted || !reflect.DeepEqual(identityIDs, []string{
		"provider-delete-identity-a",
		"provider-delete-identity-b",
	}) {
		t.Fatalf("DeleteProvider identityIDs=%v deleted=%v err=%v", identityIDs, deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, target.ID, 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE provider_id=$1`, target.ID, 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, "provider-delete-identity-a", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, "provider-delete-identity-b", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "provider-delete-binding-target", 0)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM profiles WHERE id=$1`, targetProfile.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, unrelated.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, "provider-delete-identity-unrelated", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, "provider-delete-identity-unrelated", 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "provider-delete-binding-unrelated", 1)

	identityIDs, deleted, err = db.Identities.DeleteProvider(ctx, "missing-provider")
	if err != nil || deleted || len(identityIDs) != 0 {
		t.Fatalf("missing DeleteProvider identityIDs=%v deleted=%v err=%v", identityIDs, deleted, err)
	}
}

func TestDeleteProviderRollsBackEveryRelatedDeletionOnFailureExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "provider-delete-rollback@test.com", "Password123", "ProviderDeleteRollback", false)
	provider := providerDeleteTestProvider("provider-delete-rollback", "https://rollback.identity.example")
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	item := model.ExternalIdentity{
		ID: "provider-delete-rollback-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "rollback-subject", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{IdentityID: item.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "provider_delete_rollback_profile", "ProviderDeleteRollback")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('provider-delete-rollback-binding',$1,$2,'provider-delete-rollback-remote','Remote',2,2)
	`, item.ID, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_provider_identity_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'reject provider identity delete';
		END
		$$;
		CREATE TRIGGER reject_provider_identity_delete
		BEFORE DELETE ON external_identities
		FOR EACH ROW EXECUTE FUNCTION reject_provider_identity_delete();
	`); err != nil {
		t.Fatal(err)
	}

	identityIDs, deleted, err := db.Identities.DeleteProvider(ctx, provider.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || deleted || identityIDs != nil {
		t.Fatalf("failed DeleteProvider identityIDs=%v deleted=%v err=%#v", identityIDs, deleted, err)
	}
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, provider.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, item.ID, 1)
	assertProviderDeleteCount(t, db, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, "provider-delete-rollback-binding", 1)
}

func providerDeleteTestProvider(id, issuer string) model.IdentityProvider {
	return model.IdentityProvider{
		ID: id, Name: id, IssuerURL: issuer, AuthorizationEndpoint: issuer + "/authorize",
		TokenEndpoint: issuer + "/token", JWKSURI: issuer + "/jwks", ClientID: id + "-client",
		Scopes: []string{"openid"}, Adapter: "generic_oidc", Enabled: true,
		LoginEnabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
}

func assertProviderDeleteCount(t *testing.T, db *database.DB, query, argument string, want int) {
	t.Helper()
	var got int
	err := db.Pool.QueryRow(context.Background(), query, argument).Scan(&got)
	if err != nil || got != want {
		t.Fatalf("row count=%d want=%d err=%v query=%q argument=%q", got, want, err, query, argument)
	}
}
