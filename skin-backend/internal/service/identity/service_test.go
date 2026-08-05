package identity_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type fixedDiscovery struct {
	metadata identity.ProviderMetadata
	err      error
	issuer   string
}

func (d *fixedDiscovery) Discover(_ context.Context, issuer string) (identity.ProviderMetadata, error) {
	d.issuer = issuer
	return d.metadata, d.err
}

func TestProviderAndMultipleIdentityLifecycleUsesOneModelExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "identity-admin@test.com", "pw", "IdentityAdmin", true)
	user := testutil.CreateUser(t, db, "identity-user@test.com", "pw", "IdentityUser", false)
	adminActor := actorForUser(t, db, admin.ID)
	userActor := actorForUser(t, db, user.ID)
	discovery := &fixedDiscovery{metadata: identity.ProviderMetadata{
		Issuer:                "https://issuer.example",
		AuthorizationEndpoint: "https://issuer.example/authorize",
		TokenEndpoint:         "https://issuer.example/token",
		UserInfoEndpoint:      "https://issuer.example/userinfo",
		JWKSURI:               "https://issuer.example/jwks",
	}}
	cfg := testutil.TestConfig()
	service := identity.Service{DB: db, Config: cfg, Discovery: discovery}
	secret := "provider-secret"
	created, err := service.CreateProvider(ctx, adminActor, identity.ProviderInput{
		Name:                "Example ID",
		IssuerURL:           "https://issuer.example",
		ClientID:            "client-1",
		ClientSecret:        &secret,
		Scopes:              []string{"email profile", "profile"},
		Adapter:             identity.AdapterGenericOIDC,
		Enabled:             true,
		LoginEnabled:        true,
		LinkEnabled:         true,
		RegistrationEnabled: true,
		DisplayOrder:        3,
	})
	if err != nil {
		t.Fatal(err)
	}
	providerID := created["id"].(string)
	if discovery.issuer != "https://issuer.example" || created["has_client_secret"] != true ||
		!reflect.DeepEqual(created["scopes"], []string{"email", "openid", "profile"}) {
		t.Fatalf("created provider mismatch: issuer=%q response=%#v", discovery.issuer, created)
	}
	storedProvider, err := db.Identities.GetProvider(ctx, providerID)
	if err != nil {
		t.Fatal(err)
	}
	if storedProvider == nil || storedProvider.ClientSecretCiphertext == "" || storedProvider.ClientSecretCiphertext == secret {
		t.Fatalf("provider secret must be encrypted at rest: %#v", storedProvider)
	}
	box, _ := util.NewSecretBox(cfg.IdentityEncryptionKey)
	decrypted, err := box.Decrypt(storedProvider.ClientSecretCiphertext)
	if err != nil || decrypted != secret {
		t.Fatalf("stored secret mismatch decrypted=%q err=%v", decrypted, err)
	}

	for i, subject := range []string{"subject-a", "subject-b"} {
		identityID := "identity-" + subject
		item := model.ExternalIdentity{
			ID: identityID, UserID: user.ID, ProviderID: providerID, Subject: subject,
			Label: "account", Email: subject + "@example.com", EmailVerified: true,
			DisplayName: "Display", CreatedAt: int64(100 + i), UpdatedAt: int64(100 + i),
		}
		credential := model.ExternalIdentityCredential{IdentityID: identityID, GrantedScopes: []string{"openid"}, UpdatedAt: int64(100 + i)}
		if err := db.Identities.CreateIdentity(ctx, item, credential); err != nil {
			t.Fatal(err)
		}
	}
	items, err := service.ListIdentities(ctx, userActor)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0]["subject"] != "subject-a" || items[1]["subject"] != "subject-b" ||
		items[0]["provider_id"] != providerID || items[1]["provider_id"] != providerID {
		t.Fatalf("same-provider identities should remain distinct and ordered: %#v", items)
	}
	if err := service.UpdateIdentityLabel(ctx, userActor, "identity-subject-a", "  personal  "); err != nil {
		t.Fatal(err)
	}
	updated, _ := db.Identities.GetIdentity(ctx, "identity-subject-a")
	if updated == nil || updated.Label != "personal" {
		t.Fatalf("identity label mismatch: %#v", updated)
	}
	if err := service.DeleteIdentity(ctx, userActor, "identity-subject-a"); err != nil {
		t.Fatal(err)
	}
	remaining, _ := db.Identities.ListIdentitiesByUser(ctx, user.ID)
	if len(remaining) != 1 || remaining[0].Subject != "subject-b" {
		t.Fatalf("identity deletion mismatch: %#v", remaining)
	}
}

func TestIdentityAndProviderDeletionRejectDependenciesWithExactConflicts(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "identity-dependency-admin@test.com", "pw", "IdentityDependencyAdmin", true)
	user := testutil.CreateUser(t, db, "identity-dependency-user@test.com", "pw", "IdentityDependencyUser", false)
	adminActor := actorForUser(t, db, admin.ID)
	userActor := actorForUser(t, db, user.ID)
	provider := model.IdentityProvider{
		ID: "provider-dependency", Name: "Microsoft", IssuerURL: "https://login.microsoftonline.com/consumers/v2.0",
		AuthorizationEndpoint: "https://login.microsoftonline.com/authorize", TokenEndpoint: "https://login.microsoftonline.com/token",
		JWKSURI: "https://login.microsoftonline.com/keys", ClientID: "client", Scopes: []string{"openid"}, Adapter: identity.AdapterMicrosoft,
		Enabled: true, LoginEnabled: true, LinkEnabled: true, RegistrationEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	external := model.ExternalIdentity{ID: "identity-dependency", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 2, UpdatedAt: 2}
	if err := db.Identities.CreateIdentity(ctx, external, model.ExternalIdentityCredential{IdentityID: external.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "profile-dependency", "IdentityDependencyProfile")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('binding-dependency',$1,$2,'remote-uuid','RemoteName',3,3)
	`, external.ID, profile.ID); err != nil {
		t.Fatal(err)
	}
	service := identity.Service{DB: db}
	assertHTTPError(t, service.DeleteIdentity(ctx, userActor, external.ID), 409,
		"external identity is still used by an official profile binding")
	assertHTTPError(t, service.DeleteProvider(ctx, adminActor, provider.ID), 409,
		"identity provider is still referenced by external identities")
	var identityCount, credentialCount, bindingCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, external.ID).Scan(&identityCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, external.ID).Scan(&credentialCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM official_profile_bindings WHERE id='binding-dependency'`).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if identityCount != 1 || credentialCount != 1 || bindingCount != 1 {
		t.Fatalf("failed deletions must not mutate state: identities=%d credentials=%d bindings=%d", identityCount, credentialCount, bindingCount)
	}
}

func actorForUser(t *testing.T, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(context.Background(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Detail != detail {
		t.Fatalf("HTTP error mismatch: got=%#v want status=%d detail=%q", err, status, detail)
	}
}
