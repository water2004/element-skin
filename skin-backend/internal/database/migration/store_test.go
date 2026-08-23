package migration_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/migration"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestFinalizeLegacyMicrosoftMigrationRejectsChangedSettingsWithoutPartialState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	for key, value := range map[string]string{
		"microsoft_client_id":     "legacy-client",
		"microsoft_client_secret": "legacy-secret",
		"microsoft_redirect_uri":  "https://old.example/callback",
	} {
		if err := db.Settings.Set(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := db.Migrations.ReadLegacyMicrosoftSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "microsoft_client_secret", "rotated-secret"); err != nil {
		t.Fatal(err)
	}
	provider := model.IdentityProvider{
		ID: "migrated-provider", Name: "Microsoft", IssuerURL: "https://issuer.example",
		AuthorizationEndpoint: "https://issuer.example/authorize",
		TokenEndpoint:         "https://issuer.example/token", JWKSURI: "https://issuer.example/jwks",
		ClientID: "legacy-client", Scopes: []string{"openid"}, Adapter: "microsoft",
		Enabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}

	result, err := db.Migrations.FinalizeLegacyMicrosoftMigration(ctx, expected, &provider)
	if !errors.Is(err, migration.ErrLegacyMicrosoftSettingsChanged) ||
		result.ProviderCreated || result.LegacySettingsRemoved {
		t.Fatalf("changed settings result mismatch: result=%#v err=%v", result, err)
	}
	if value, err := db.Settings.Get(ctx, "microsoft_client_secret", "missing"); err != nil || value != "rotated-secret" {
		t.Fatalf("changed secret was not preserved: value=%q err=%v", value, err)
	}
	var providerCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers`).Scan(&providerCount); err != nil || providerCount != 0 {
		t.Fatalf("provider state was polluted: count=%d err=%v", providerCount, err)
	}
}
