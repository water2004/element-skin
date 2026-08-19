package identity_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestLegacyMicrosoftConfigurationMigratesExactlyOnceAndPreservesProfiles(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	setLegacyMicrosoftSettings(t, db, "legacy-client", " legacy-secret ", "https://old.example/callback")
	user := testutil.CreateUser(t, db, "legacy-microsoft@test.com", "pw", "LegacyMicrosoft", false)
	profile := testutil.CreateProfile(t, db, user.ID, "0123456789abcdef0123456789abcdef", "LegacyProfile")
	discovery := microsoftDiscovery()
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}

	result, err := service.MigrateLegacyMicrosoftProvider(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ProviderCreated || !result.LegacySettingsRemoved {
		t.Fatalf("migration result mismatch: %#v", result)
	}
	providers, err := db.Identities.ListProviders(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 {
		t.Fatalf("provider count mismatch: %#v", providers)
	}
	provider := providers[0]
	if provider.Name != "Microsoft" || provider.IssuerURL != identity.MicrosoftConsumerIssuer ||
		provider.ClientID != "legacy-client" || provider.Adapter != identity.AdapterMicrosoft ||
		!provider.Enabled || provider.LoginEnabled || !provider.LinkEnabled ||
		!reflect.DeepEqual(provider.Scopes, []string{"XboxLive.signin", "email", "offline_access", "openid", "profile"}) {
		t.Fatalf("migrated provider mismatch: %#v", provider)
	}
	box, err := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := box.Decrypt(provider.ClientSecretCiphertext)
	if err != nil || decrypted != "legacy-secret" {
		t.Fatalf("migrated provider secret mismatch: decrypted=%q err=%v", decrypted, err)
	}
	assertLegacyMicrosoftSettingsRemoved(t, db)
	persisted, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || persisted == nil || persisted.UserID != user.ID || persisted.Name != "LegacyProfile" {
		t.Fatalf("legacy imported profile changed: profile=%#v err=%v", persisted, err)
	}

	second, err := service.MigrateLegacyMicrosoftProvider(ctx)
	if err != nil || second.ProviderCreated || second.LegacySettingsRemoved {
		t.Fatalf("second migration must be a no-op: result=%#v err=%v", second, err)
	}
	providers, err = db.Identities.ListProviders(ctx, false)
	if err != nil || len(providers) != 1 {
		t.Fatalf("idempotent provider count mismatch: providers=%#v err=%v", providers, err)
	}
}

func TestLegacyMicrosoftMigrationRemovesOnlyUnusedDefaultsWithoutDiscovery(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	setLegacyMicrosoftSettings(t, db, "", "", "http://localhost:8000/v1/imports/microsoft/callback")
	discovery := &fixedDiscovery{err: errors.New("discovery must not run")}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}

	result, err := service.MigrateLegacyMicrosoftProvider(ctx)
	if err != nil || result.ProviderCreated || !result.LegacySettingsRemoved || discovery.issuer != "" {
		t.Fatalf("unused migration mismatch: result=%#v issuer=%q err=%v", result, discovery.issuer, err)
	}
	assertLegacyMicrosoftSettingsRemoved(t, db)
	providers, err := db.Identities.ListProviders(ctx, false)
	if err != nil || len(providers) != 0 {
		t.Fatalf("unused settings created provider: providers=%#v err=%v", providers, err)
	}
}

func TestLegacyMicrosoftMigrationFailuresPreserveSettingsAndProviderState(t *testing.T) {
	t.Run("incomplete credentials", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		setLegacyMicrosoftSettings(t, db, "legacy-client", "", "https://old.example/callback")
		discovery := &fixedDiscovery{err: errors.New("discovery must not run for incomplete credentials")}
		service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}

		result, err := service.MigrateLegacyMicrosoftProvider(context.Background())
		if result.ProviderCreated || result.LegacySettingsRemoved || err == nil ||
			err.Error() != "migrate legacy Microsoft configuration: client_id and client_secret must both be configured" ||
			discovery.issuer != "" {
			t.Fatalf("incomplete migration mismatch: result=%#v err=%v", result, err)
		}
		assertLegacyMicrosoftSettingsExact(t, db, "legacy-client", "", "https://old.example/callback")
		assertProviderCount(t, db, 0)
	})

	t.Run("discovery failure", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		setLegacyMicrosoftSettings(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		service := identity.Service{
			DB: db, Config: testutil.TestConfig(),
			Discovery: &fixedDiscovery{err: errors.New("discovery unavailable")},
		}

		result, err := service.MigrateLegacyMicrosoftProvider(context.Background())
		if result.ProviderCreated || result.LegacySettingsRemoved || err == nil ||
			err.Error() != "migrate legacy Microsoft configuration: identity_provider.discover.failed" {
			t.Fatalf("discovery migration mismatch: result=%#v err=%v", result, err)
		}
		assertLegacyMicrosoftSettingsExact(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		assertProviderCount(t, db, 0)
	})

	t.Run("invalid encryption key", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		setLegacyMicrosoftSettings(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		cfg := testutil.TestConfig()
		cfg.IdentityEncryptionKey = "not-base64"
		service := identity.Service{DB: db, Config: cfg, Discovery: microsoftDiscovery()}

		result, err := service.MigrateLegacyMicrosoftProvider(context.Background())
		if result.ProviderCreated || result.LegacySettingsRemoved || err == nil ||
			!strings.Contains(err.Error(), "decode identity encryption key") {
			t.Fatalf("encryption migration mismatch: result=%#v err=%v", result, err)
		}
		assertLegacyMicrosoftSettingsExact(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		assertProviderCount(t, db, 0)
	})

	t.Run("provider conflict", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		setLegacyMicrosoftSettings(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		existing := model.IdentityProvider{
			ID: "generic-provider", Name: "Generic", IssuerURL: identity.MicrosoftConsumerIssuer,
			AuthorizationEndpoint: "https://issuer.example/authorize",
			TokenEndpoint:         "https://issuer.example/token", JWKSURI: "https://issuer.example/jwks",
			ClientID: "legacy-client", Scopes: []string{"openid"}, Adapter: identity.AdapterGenericOIDC,
			Enabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
		}
		if err := db.Identities.CreateProvider(ctx, existing); err != nil {
			t.Fatal(err)
		}
		discovery := &fixedDiscovery{err: errors.New("discovery must not run for an existing provider")}
		service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}

		result, err := service.MigrateLegacyMicrosoftProvider(ctx)
		if result.ProviderCreated || result.LegacySettingsRemoved || err == nil ||
			!strings.Contains(err.Error(), "conflicts with a non-Microsoft identity provider") || discovery.issuer != "" {
			t.Fatalf("conflict migration mismatch: result=%#v err=%v", result, err)
		}
		assertLegacyMicrosoftSettingsExact(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
		assertProviderCount(t, db, 1)
	})
}

func TestLegacyMicrosoftMigrationReusesExistingMicrosoftProviderWithoutDuplication(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	setLegacyMicrosoftSettings(t, db, "legacy-client", "legacy-secret", "https://old.example/callback")
	existing := model.IdentityProvider{
		ID: "existing-microsoft", Name: "Existing Microsoft", IssuerURL: identity.MicrosoftConsumerIssuer,
		AuthorizationEndpoint: "https://issuer.example/authorize",
		TokenEndpoint:         "https://issuer.example/token", JWKSURI: "https://issuer.example/jwks",
		ClientID: "legacy-client", ClientSecretCiphertext: "existing-ciphertext",
		Scopes: []string{"openid"}, Adapter: identity.AdapterMicrosoft,
		Enabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, existing); err != nil {
		t.Fatal(err)
	}
	discovery := &fixedDiscovery{err: errors.New("discovery must not run for an existing provider")}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Discovery: discovery}

	result, err := service.MigrateLegacyMicrosoftProvider(ctx)
	if err != nil || result.ProviderCreated || !result.LegacySettingsRemoved || discovery.issuer != "" {
		t.Fatalf("existing provider migration mismatch: result=%#v err=%v", result, err)
	}
	assertLegacyMicrosoftSettingsRemoved(t, db)
	providers, err := db.Identities.ListProviders(ctx, false)
	if err != nil || len(providers) != 1 || providers[0].ID != existing.ID ||
		providers[0].ClientSecretCiphertext != existing.ClientSecretCiphertext {
		t.Fatalf("existing provider changed: providers=%#v err=%v", providers, err)
	}
}

func microsoftDiscovery() *fixedDiscovery {
	return &fixedDiscovery{metadata: identity.ProviderMetadata{
		Issuer:                identity.MicrosoftConsumerIssuer,
		AuthorizationEndpoint: "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
		TokenEndpoint:         "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		UserInfoEndpoint:      "https://graph.microsoft.com/oidc/userinfo",
		JWKSURI:               "https://login.microsoftonline.com/consumers/discovery/v2.0/keys",
	}}
}

func setLegacyMicrosoftSettings(t *testing.T, db *database.DB, clientID, clientSecret, redirectURI string) {
	t.Helper()
	ctx := context.Background()
	for key, value := range map[string]string{
		"microsoft_client_id":     clientID,
		"microsoft_client_secret": clientSecret,
		"microsoft_redirect_uri":  redirectURI,
	} {
		if err := db.Settings.Set(ctx, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
}

func assertLegacyMicrosoftSettingsRemoved(t *testing.T, db *database.DB) {
	t.Helper()
	ctx := context.Background()
	for _, key := range []string{"microsoft_client_id", "microsoft_client_secret", "microsoft_redirect_uri"} {
		value, err := db.Settings.Get(ctx, key, "missing")
		if err != nil || value != "missing" {
			t.Fatalf("legacy setting %s not removed: value=%q err=%v", key, value, err)
		}
	}
}

func assertLegacyMicrosoftSettingsExact(
	t *testing.T,
	db *database.DB,
	clientID, clientSecret, redirectURI string,
) {
	t.Helper()
	ctx := context.Background()
	want := map[string]string{
		"microsoft_client_id":     clientID,
		"microsoft_client_secret": clientSecret,
		"microsoft_redirect_uri":  redirectURI,
	}
	for key, expected := range want {
		value, err := db.Settings.Get(ctx, key, "missing")
		if err != nil || value != expected {
			t.Fatalf("legacy setting %s mismatch: value=%q want=%q err=%v", key, value, expected, err)
		}
	}
}

func assertProviderCount(t *testing.T, db *database.DB, expected int) {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM identity_providers`).Scan(&count); err != nil || count != expected {
		t.Fatalf("provider count mismatch: count=%d want=%d err=%v", count, expected, err)
	}
}
