package identity_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
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

func TestMicrosoftProviderRequiresMinecraftAndRefreshScopesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "microsoft-scope-admin@test.com", "pw", "MicrosoftScopeAdmin", true)
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: &fixedDiscovery{}}
	secret := "secret"
	_, err := service.CreateProvider(ctx, actorForUser(t, db, admin.ID), identity.ProviderInput{
		Name: "Microsoft", IssuerURL: "https://login.example", ClientID: "client", ClientSecret: &secret,
		Scopes: []string{"openid", "profile"}, Adapter: identity.AdapterMicrosoft, Enabled: true,
	})
	assertHTTPError(t, err, 400, "Microsoft providers must request XboxLive.signin and offline_access scopes")
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid Microsoft provider persisted count=%d err=%v", count, err)
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

func TestProviderManagementQueriesUpdatesAndRejectsInvalidStateExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "identity-management-admin@test.com", "pw", "IdentityManagementAdmin", true)
	adminActor := actorForUser(t, db, admin.ID)
	discovery := &fixedDiscovery{metadata: validProviderMetadata("https://management.example")}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}
	secret := "management-secret"
	input := identity.ProviderInput{
		Name: "Management Provider", IssuerURL: "https://management.example", ClientID: "management-client",
		ClientSecret: &secret, Scopes: []string{"profile", "email"}, Adapter: identity.AdapterGenericOIDC,
		IconURL: "https://management.example/icon.png", Enabled: true, LoginEnabled: true, LinkEnabled: true,
		RegistrationEnabled: true, DisplayOrder: 4,
	}
	created, err := service.CreateProvider(ctx, adminActor, input)
	if err != nil {
		t.Fatal(err)
	}
	providerID := created["id"].(string)

	got, err := service.GetProvider(ctx, adminActor, "  "+providerID+"  ")
	if err != nil || got["id"] != providerID || got["name"] != input.Name || got["has_client_secret"] != true {
		t.Fatalf("provider detail=%#v err=%v", got, err)
	}
	listed, err := service.ListProviders(ctx, adminActor)
	if err != nil || len(listed) != 1 || listed[0]["id"] != providerID {
		t.Fatalf("admin provider list=%#v err=%v", listed, err)
	}
	public, err := service.ListPublicProviders(ctx, permission.GuestActor())
	if err != nil || len(public) != 1 || len(public[0]) != 7 || public[0]["id"] != providerID {
		t.Fatalf("public provider list=%#v err=%v", public, err)
	}

	input.Name = "Updated Management Provider"
	input.ClientSecret = nil
	input.Scopes = []string{"openid", "groups"}
	input.DisplayOrder = 9
	updated, err := service.UpdateProvider(ctx, adminActor, providerID, input)
	if err != nil || updated["name"] != input.Name || updated["display_order"] != 9 ||
		!reflect.DeepEqual(updated["scopes"], []string{"groups", "openid"}) || updated["has_client_secret"] != true {
		t.Fatalf("updated provider=%#v err=%v", updated, err)
	}
	stored, err := db.Identities.GetProvider(ctx, providerID)
	if err != nil || stored == nil || stored.ClientSecretCiphertext == "" {
		t.Fatalf("updated provider lost encrypted secret: provider=%#v err=%v", stored, err)
	}

	duplicateSecret := "duplicate-secret"
	duplicateInput := input
	duplicateInput.Name = "Duplicate"
	duplicateInput.ClientSecret = &duplicateSecret
	_, err = service.CreateProvider(ctx, adminActor, duplicateInput)
	assertHTTPError(t, err, 409, "an identity provider with this issuer and client_id already exists")
	if _, err := service.GetProvider(ctx, adminActor, "missing-provider"); err == nil {
		t.Fatal("missing provider detail should fail")
	} else {
		assertHTTPError(t, err, 404, "identity provider not found")
	}
	if _, err := service.UpdateProvider(ctx, adminActor, "missing-provider", input); err == nil {
		t.Fatal("missing provider update should fail")
	} else {
		assertHTTPError(t, err, 404, "identity provider not found")
	}
	assertHTTPError(t, service.DeleteProvider(ctx, adminActor, "missing-provider"), 404, "identity provider not found")

	for name, call := range map[string]func() error{
		"list": func() error { _, err := service.ListProviders(ctx, permission.Actor{}); return err },
		"get":  func() error { _, err := service.GetProvider(ctx, permission.Actor{}, providerID); return err },
		"create": func() error {
			_, err := service.CreateProvider(ctx, permission.Actor{}, input)
			return err
		},
		"update": func() error {
			_, err := service.UpdateProvider(ctx, permission.Actor{}, providerID, input)
			return err
		},
		"delete": func() error { return service.DeleteProvider(ctx, permission.Actor{}, providerID) },
	} {
		t.Run("permission_"+name, func(t *testing.T) {
			assertHTTPError(t, call(), 403, "permission denied")
		})
	}

	assertHTTPError(t, service.UpdateIdentityLabel(ctx, adminActor, "missing", strings.Repeat("界", 81)), 400,
		"identity label must not exceed 80 characters")
	assertHTTPError(t, service.UpdateIdentityLabel(ctx, adminActor, "missing", "valid"), 404,
		"external identity not found")
	assertHTTPError(t, service.DeleteIdentity(ctx, adminActor, "missing"), 404, "external identity not found")
}

func TestProviderValidationRejectsEveryMalformedContractWithoutPersistence(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "identity-validation-admin@test.com", "pw", "IdentityValidationAdmin", true)
	actor := actorForUser(t, db, admin.ID)
	secret := "validation-secret"
	baseInput := func() identity.ProviderInput {
		return identity.ProviderInput{
			Name: "Validation Provider", IssuerURL: "https://validation.example", ClientID: "validation-client",
			ClientSecret: &secret, Scopes: []string{"openid"}, Adapter: identity.AdapterGenericOIDC,
			Enabled: true, LoginEnabled: true, LinkEnabled: true,
		}
	}
	tests := []struct {
		name       string
		mutate     func(*identity.ProviderInput)
		metadata   identity.ProviderMetadata
		discovery  error
		wantDetail string
	}{
		{name: "missing name", mutate: func(in *identity.ProviderInput) { in.Name = " " }, wantDetail: "provider name is required and must not exceed 80 characters"},
		{name: "long name", mutate: func(in *identity.ProviderInput) { in.Name = strings.Repeat("界", 81) }, wantDetail: "provider name is required and must not exceed 80 characters"},
		{name: "missing client", mutate: func(in *identity.ProviderInput) { in.ClientID = " " }, wantDetail: "client_id is required and must not exceed 512 characters"},
		{name: "invalid adapter", mutate: func(in *identity.ProviderInput) { in.Adapter = "saml" }, wantDetail: "invalid provider adapter"},
		{name: "insecure issuer", mutate: func(in *identity.ProviderInput) { in.IssuerURL = "http://identity.example" }, wantDetail: "invalid issuer_url"},
		{name: "invalid icon", mutate: func(in *identity.ProviderInput) { in.IconURL = "https://identity.example/icon#fragment" }, wantDetail: "invalid icon_url"},
		{name: "invalid scope", mutate: func(in *identity.ProviderInput) { in.Scopes = []string{`bad"scope`} }, wantDetail: "invalid OIDC scope"},
		{name: "too many scopes", mutate: func(in *identity.ProviderInput) {
			in.Scopes = make([]string, 33)
			for index := range in.Scopes {
				in.Scopes[index] = "scope" + strconv.Itoa(index)
			}
		}, wantDetail: "too many OIDC scopes"},
		{name: "discovery failure", discovery: errors.New("discovery unavailable"), wantDetail: "discovery unavailable"},
		{name: "issuer mismatch", metadata: validProviderMetadata("https://other.example"), wantDetail: "OIDC discovery issuer does not exactly match issuer_url"},
		{name: "invalid required endpoint", metadata: func() identity.ProviderMetadata {
			metadata := validProviderMetadata("https://validation.example")
			metadata.TokenEndpoint = "http://identity.example/token"
			return metadata
		}(), wantDetail: "OIDC discovery document contains an invalid required endpoint"},
		{name: "invalid userinfo endpoint", metadata: func() identity.ProviderMetadata {
			metadata := validProviderMetadata("https://validation.example")
			metadata.UserInfoEndpoint = "https://identity.example/userinfo?tenant=1"
			return metadata
		}(), wantDetail: "OIDC discovery document contains an invalid userinfo endpoint"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput()
			if tc.mutate != nil {
				tc.mutate(&input)
			}
			metadata := tc.metadata
			if metadata.Issuer == "" {
				metadata = validProviderMetadata(input.IssuerURL)
			}
			service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: &fixedDiscovery{metadata: metadata, err: tc.discovery}}
			created, err := service.CreateProvider(ctx, actor, input)
			if created != nil {
				t.Fatalf("invalid provider response=%#v", created)
			}
			assertHTTPError(t, err, 400, tc.wantDetail)
		})
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid providers persisted count=%d err=%v", count, err)
	}
}

func TestIdentityServicePropagatesClosedDatabaseErrorsFromEveryLifecycleEntryExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "identity-closed-admin@test.com", "pw", "IdentityClosedAdmin", true)
	user := testutil.CreateUser(t, db, "identity-closed-user@test.com", "pw", "IdentityClosedUser", false)
	adminActor := actorForUser(t, db, admin.ID)
	userActor := actorForUser(t, db, user.ID)
	discovery := &fixedDiscovery{metadata: validProviderMetadata("https://closed.example")}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}
	secret := "closed-secret"
	input := identity.ProviderInput{
		Name: "Closed Provider", IssuerURL: "https://closed.example", ClientID: "closed-client",
		ClientSecret: &secret, Scopes: []string{"openid"}, Adapter: identity.AdapterGenericOIDC, Enabled: true,
	}
	db.Close()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "list public providers", call: func() error { _, err := service.ListPublicProviders(ctx, permission.GuestActor()); return err }},
		{name: "list providers", call: func() error { _, err := service.ListProviders(ctx, adminActor); return err }},
		{name: "get provider", call: func() error { _, err := service.GetProvider(ctx, adminActor, "provider"); return err }},
		{name: "create provider", call: func() error { _, err := service.CreateProvider(ctx, adminActor, input); return err }},
		{name: "update provider", call: func() error { _, err := service.UpdateProvider(ctx, adminActor, "provider", input); return err }},
		{name: "delete provider", call: func() error { return service.DeleteProvider(ctx, adminActor, "provider") }},
		{name: "list identities", call: func() error { _, err := service.ListIdentities(ctx, userActor); return err }},
		{name: "update identity", call: func() error { return service.UpdateIdentityLabel(ctx, userActor, "identity", "label") }},
		{name: "delete identity", call: func() error { return service.DeleteIdentity(ctx, userActor, "identity") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "closed pool") {
				t.Fatalf("closed database error=%v", err)
			}
			var httpErr util.HTTPError
			if errors.As(err, &httpErr) {
				t.Fatalf("closed database error was converted to HTTP error: %#v", httpErr)
			}
		})
	}
}

func validProviderMetadata(issuer string) identity.ProviderMetadata {
	return identity.ProviderMetadata{
		Issuer: issuer, AuthorizationEndpoint: issuer + "/authorize", TokenEndpoint: issuer + "/token",
		UserInfoEndpoint: issuer + "/userinfo", JWKSURI: issuer + "/jwks",
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
