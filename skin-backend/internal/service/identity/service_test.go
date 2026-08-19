package identity_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type fixedDiscovery struct {
	metadata identity.ProviderMetadata
	err      error
	issuer   string
}

type externalTokenDeleteFailStore struct {
	redisstore.Store
}

func (s externalTokenDeleteFailStore) DeleteExternalAccessToken(context.Context, string) error {
	return errors.New("external access token deletion failed")
}

type externalTokenDeleteNthFailStore struct {
	redisstore.Store
	deleteCalls int
	failOnCall  int
}

func (s *externalTokenDeleteNthFailStore) DeleteExternalAccessToken(ctx context.Context, identityID string) error {
	s.deleteCalls++
	if s.deleteCalls == s.failOnCall {
		return errors.New("external access token deletion failed")
	}
	return s.Store.DeleteExternalAccessToken(ctx, identityID)
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
		Name:         "Example ID",
		IssuerURL:    "https://issuer.example",
		ClientID:     "client-1",
		ClientSecret: &secret,
		Scopes:       []string{"email profile", "profile"},
		Adapter:      identity.AdapterGenericOIDC,
		Enabled:      true,
		LoginEnabled: true,
		LinkEnabled:  true,
		DisplayOrder: 3,
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
	assertHTTPError(t, err, 400, "identity_provider_scope.configure.required")
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid Microsoft provider persisted count=%d err=%v", count, err)
	}
}

func TestIdentityAndProviderDeletionDetachBindingsAndPreserveProfilesExactly(t *testing.T) {
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
		Enabled: true, LoginEnabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
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
	cache := redisstore.NewMemoryStore()
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{
		IdentityID: external.ID, AccessToken: "provider-delete-access", ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	service := identity.Service{DB: db, Redis: cache}
	detachedIdentity := model.ExternalIdentity{
		ID: "identity-user-delete", UserID: user.ID, ProviderID: provider.ID,
		Subject: "user-delete-subject", CreatedAt: 4, UpdatedAt: 4,
	}
	if err := db.Identities.CreateIdentity(ctx, detachedIdentity, model.ExternalIdentityCredential{IdentityID: detachedIdentity.ID, UpdatedAt: 4}); err != nil {
		t.Fatal(err)
	}
	detachedProfile := testutil.CreateProfile(t, db, user.ID, "profile_user_identity_delete", "UserIdentityDeleteProfile")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('binding-user-identity-delete',$1,$2,'remote-user-delete','RemoteDelete',5,5)
	`, detachedIdentity.ID, detachedProfile.ID); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{
		IdentityID: detachedIdentity.ID, AccessToken: "identity-delete-access", ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteIdentity(ctx, userActor, detachedIdentity.ID); err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM external_identities WHERE id='identity-user-delete'`, 0},
		{`SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id='identity-user-delete'`, 0},
		{`SELECT COUNT(*) FROM official_profile_bindings WHERE id='binding-user-identity-delete'`, 0},
		{`SELECT COUNT(*) FROM profiles WHERE id='profile_user_identity_delete'`, 1},
	} {
		var count int
		if err := db.Pool.QueryRow(ctx, state.query).Scan(&count); err != nil || count != state.want {
			t.Fatalf("identity deletion state count=%d want=%d err=%v query=%q", count, state.want, err, state.query)
		}
	}
	if _, err := cache.GetExternalAccessToken(ctx, detachedIdentity.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("identity deletion must remove cached external access token: %v", err)
	}
	failingService := identity.Service{DB: db, Redis: externalTokenDeleteFailStore{Store: cache}}
	if err := failingService.DeleteProvider(ctx, adminActor, provider.ID); err == nil || err.Error() != "external access token deletion failed" {
		t.Fatalf("provider cache cleanup failure mismatch: %#v", err)
	}
	var preservedProviderCount, preservedIdentityCount, preservedCredentialCount, preservedBindingCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, provider.ID).Scan(&preservedProviderCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, external.ID).Scan(&preservedIdentityCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, external.ID).Scan(&preservedCredentialCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM official_profile_bindings WHERE id='binding-dependency'`).Scan(&preservedBindingCount); err != nil {
		t.Fatal(err)
	}
	if preservedProviderCount != 1 || preservedIdentityCount != 1 || preservedCredentialCount != 1 || preservedBindingCount != 1 {
		t.Fatalf("cache cleanup failure mutated database: providers=%d identities=%d credentials=%d bindings=%d",
			preservedProviderCount, preservedIdentityCount, preservedCredentialCount, preservedBindingCount)
	}
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{
		IdentityID: external.ID, AccessToken: "provider-delete-access", ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	postCommitCache := &externalTokenDeleteNthFailStore{Store: cache, failOnCall: 2}
	postCommitService := identity.Service{DB: db, Redis: postCommitCache}
	if err := postCommitService.DeleteProvider(ctx, adminActor, provider.ID); err != nil {
		t.Fatal(err)
	}
	if postCommitCache.deleteCalls != 2 {
		t.Fatalf("provider cache cleanup calls=%d want=2", postCommitCache.deleteCalls)
	}
	var providerCount, identityCount, credentialCount, bindingCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, provider.ID).Scan(&providerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identities WHERE id=$1`, external.ID).Scan(&identityCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identity_credentials WHERE identity_id=$1`, external.ID).Scan(&credentialCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM official_profile_bindings WHERE id='binding-dependency'`).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if providerCount != 0 || identityCount != 0 || credentialCount != 0 || bindingCount != 0 {
		t.Fatalf("provider deletion residues: providers=%d identities=%d credentials=%d bindings=%d", providerCount, identityCount, credentialCount, bindingCount)
	}
	if storedProfile, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || storedProfile == nil || storedProfile.UserID != user.ID {
		t.Fatalf("provider deletion must preserve the local profile: profile=%#v err=%v", storedProfile, err)
	}
	if _, err := cache.GetExternalAccessToken(ctx, external.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("provider deletion must remove cached external access token: %v", err)
	}
}

func TestDeleteIdentityCacheFailureSemanticsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "identity-cache-user@test.com", "pw", "IdentityCacheUser", false)
	actor := actorForUser(t, db, user.ID)
	provider := model.IdentityProvider{
		ID: "provider-identity-cache", Name: "Identity Cache", IssuerURL: "https://identity-cache.example",
		AuthorizationEndpoint: "https://identity-cache.example/authorize", TokenEndpoint: "https://identity-cache.example/token",
		JWKSURI: "https://identity-cache.example/keys", ClientID: "client", Scopes: []string{"openid"},
		Adapter: identity.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	createIdentity := func(id, subject string) {
		t.Helper()
		item := model.ExternalIdentity{
			ID: id, UserID: user.ID, ProviderID: provider.ID, Subject: subject, CreatedAt: 2, UpdatedAt: 2,
		}
		credential := model.ExternalIdentityCredential{IdentityID: id, UpdatedAt: 2}
		if err := db.Identities.CreateIdentity(ctx, item, credential); err != nil {
			t.Fatal(err)
		}
	}
	cache := redisstore.NewMemoryStore()
	setToken := func(identityID, accessToken string) {
		t.Helper()
		if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{
			IdentityID: identityID, AccessToken: accessToken, ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
		}, time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	createIdentity("identity-cache-precommit", "subject-precommit")
	setToken("identity-cache-precommit", "precommit-access")
	preCommitService := identity.Service{DB: db, Redis: externalTokenDeleteFailStore{Store: cache}}
	err := preCommitService.DeleteIdentity(ctx, actor, "identity-cache-precommit")
	if err == nil || err.Error() != "external access token deletion failed" {
		t.Fatalf("pre-commit cache failure=%v", err)
	}
	if stored, getErr := db.Identities.GetIdentity(ctx, "identity-cache-precommit"); getErr != nil || stored == nil {
		t.Fatalf("pre-commit cache failure removed identity: identity=%#v err=%v", stored, getErr)
	}
	if token, getErr := cache.GetExternalAccessToken(ctx, "identity-cache-precommit"); getErr != nil || token.AccessToken != "precommit-access" {
		t.Fatalf("pre-commit cache failure changed token: token=%#v err=%v", token, getErr)
	}

	createIdentity("identity-cache-postcommit", "subject-postcommit")
	setToken("identity-cache-postcommit", "postcommit-access")
	postCommitCache := &externalTokenDeleteNthFailStore{Store: cache, failOnCall: 2}
	postCommitService := identity.Service{DB: db, Redis: postCommitCache}
	if err := postCommitService.DeleteIdentity(ctx, actor, "identity-cache-postcommit"); err != nil {
		t.Fatal(err)
	}
	if postCommitCache.deleteCalls != 2 {
		t.Fatalf("identity cache cleanup calls=%d want=2", postCommitCache.deleteCalls)
	}
	if stored, getErr := db.Identities.GetIdentity(ctx, "identity-cache-postcommit"); getErr != nil || stored != nil {
		t.Fatalf("post-commit cache failure identity=%#v err=%v", stored, getErr)
	}
	if _, getErr := cache.GetExternalAccessToken(ctx, "identity-cache-postcommit"); !errors.Is(getErr, redisstore.ErrCacheMiss) {
		t.Fatalf("post-commit cache failure left token: %v", getErr)
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
		DisplayOrder: 4,
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
	if err != nil || len(public) != 1 || len(public[0]) != 6 || public[0]["id"] != providerID {
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
	assertHTTPError(t, err, 409, "identity_provider.create.conflict")
	if _, err := service.GetProvider(ctx, adminActor, "missing-provider"); err == nil {
		t.Fatal("missing provider detail should fail")
	} else {
		assertHTTPError(t, err, 404, "identity_provider.resolve.not_found")
	}
	if _, err := service.UpdateProvider(ctx, adminActor, "missing-provider", input); err == nil {
		t.Fatal("missing provider update should fail")
	} else {
		assertHTTPError(t, err, 404, "identity_provider.resolve.not_found")
	}
	assertHTTPError(t, service.DeleteProvider(ctx, adminActor, "missing-provider"), 404, "identity_provider.resolve.not_found")

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
			assertHTTPError(t, call(), 403, "permission.check.denied")
		})
	}

	assertHTTPError(t, service.UpdateIdentityLabel(ctx, adminActor, "missing", strings.Repeat("界", 81)), 400,
		"identity_label.validate.too_long")
	assertHTTPError(t, service.UpdateIdentityLabel(ctx, adminActor, "missing", "valid"), 404,
		"identity.resolve.not_found")
	assertHTTPError(t, service.DeleteIdentity(ctx, adminActor, "missing"), 404, "identity.resolve.not_found")
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
		name               string
		mutate             func(*identity.ProviderInput)
		metadata           identity.ProviderMetadata
		discovery          error
		wantClassification string
	}{
		{name: "missing name", mutate: func(in *identity.ProviderInput) { in.Name = " " }, wantClassification: "identity_provider_name.validate.invalid"},
		{name: "long name", mutate: func(in *identity.ProviderInput) { in.Name = strings.Repeat("界", 81) }, wantClassification: "identity_provider_name.validate.invalid"},
		{name: "missing client", mutate: func(in *identity.ProviderInput) { in.ClientID = " " }, wantClassification: "client_id.validate.invalid"},
		{name: "invalid adapter", mutate: func(in *identity.ProviderInput) { in.Adapter = "saml" }, wantClassification: "identity_provider_adapter.validate.invalid"},
		{name: "insecure issuer", mutate: func(in *identity.ProviderInput) { in.IssuerURL = "http://identity.example" }, wantClassification: "issuer_url.validate.invalid"},
		{name: "invalid icon", mutate: func(in *identity.ProviderInput) { in.IconURL = "https://identity.example/icon#fragment" }, wantClassification: "icon_url.validate.invalid"},
		{name: "oauth_scope.validate.invalid", mutate: func(in *identity.ProviderInput) { in.Scopes = []string{`bad"scope`} }, wantClassification: "oidc_scope.validate.invalid"},
		{name: "too many scopes", mutate: func(in *identity.ProviderInput) {
			in.Scopes = make([]string, 33)
			for index := range in.Scopes {
				in.Scopes[index] = "scope" + strconv.Itoa(index)
			}
		}, wantClassification: "oidc_scope.validate.exceeded"},
		{name: "discovery failure", discovery: errors.New("discovery unavailable"), wantClassification: "identity_provider.discover.failed"},
		{name: "issuer mismatch", metadata: validProviderMetadata("https://other.example"), wantClassification: "identity_provider.discover.mismatch"},
		{name: "invalid required endpoint", metadata: func() identity.ProviderMetadata {
			metadata := validProviderMetadata("https://validation.example")
			metadata.TokenEndpoint = "http://identity.example/token"
			return metadata
		}(), wantClassification: "identity_provider.discover.invalid"},
		{name: "invalid userinfo endpoint", metadata: func() identity.ProviderMetadata {
			metadata := validProviderMetadata("https://validation.example")
			metadata.UserInfoEndpoint = "https://identity.example/userinfo?tenant=1"
			return metadata
		}(), wantClassification: "identity_provider.discover.invalid"},
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
			assertHTTPError(t, err, 400, tc.wantClassification)
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
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Error() != detail {
		t.Fatalf("HTTP error mismatch: got=%#v want status=%d detail=%q", err, status, detail)
	}
}
