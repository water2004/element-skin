package database_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestDBInitSchemaDefaultsAndCoreHelpers(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Init should be idempotent: %v", err)
	}
	for _, table := range []string{
		"users",
		"profiles",
		"site_refresh_tokens",
		"invites",
		"settings",
		"email_suffix_policy",
		"email_suffix_rules",
		"user_textures",
		"skin_library",
		"fallback_endpoints",
		"fallback_skin_domains",
		"whitelisted_users",
		"verification_codes",
		"enabled_easter_eggs",
		"identity_providers",
		"external_identities",
		"external_identity_credentials",
		"official_profile_bindings",
		"delegated_clients",
		"delegated_client_permissions",
		"delegated_permission_grants",
		"delegated_grant_permissions",
		"oauth_authorization_codes",
		"oauth_authorization_code_permissions",
		"oauth_refresh_tokens",
		"webhook_endpoints",
		"webhook_endpoint_events",
		"webhook_active_event_types",
		"webhook_events",
		"webhook_deliveries",
		"permission_audit_logs",
	} {
		var exists bool
		if err := db.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("InitSQL should create table %s", table)
		}
	}
	siteName, err := db.Settings.Get(ctx, "site_name", "")
	if err != nil {
		t.Fatal(err)
	}
	if siteName != "皮肤站" {
		t.Fatalf("InitSQL should seed site_name, got %q", siteName)
	}

	avatar := "avatar_hash"
	user := testutil.CreateUser(t, db, "scan@test.com", "Password123", "ScanUser", true)
	if err := db.Users.Update(ctx, user.ID, map[string]any{"avatar_hash": avatar}); err != nil {
		t.Fatal(err)
	}
	scannedUser, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scannedUser == nil || scannedUser.ID != user.ID || scannedUser.Email != user.Email ||
		scannedUser.PreferredLanguage != "zh_CN" || scannedUser.DisplayName != "ScanUser" || scannedUser.BannedUntil != nil ||
		scannedUser.AvatarHash == nil || *scannedUser.AvatarHash != "avatar_hash" {
		t.Fatalf("GetUserByID/scanUser mismatch: %#v", scannedUser)
	}

	skin := "skin"
	profile := testutil.CreateProfile(t, db, user.ID, "scan_profile", "ScanProfile")
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	scannedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scannedProfile == nil || scannedProfile.ID != profile.ID || scannedProfile.UserID != user.ID || scannedProfile.Name != "ScanProfile" ||
		scannedProfile.TextureModel != "slim" || scannedProfile.SkinHash == nil || *scannedProfile.SkinHash != "skin" || scannedProfile.CapeHash != nil {
		t.Fatalf("GetProfileByID/scanProfile mismatch: %#v", scannedProfile)
	}
	if !database.IsNoRows(pgx.ErrNoRows) || database.IsNoRows(nil) {
		t.Fatalf("IsNoRows should match pgx.ErrNoRows only")
	}
	before := database.NowMS()
	after := database.NowMS()
	if before <= 0 || after < before {
		t.Fatalf("NowMS should be positive and monotonic enough: before=%d after=%d", before, after)
	}
}

func TestResetPublicSchemaRemovesDataAndRestoresDefaults(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	testutil.CreateUser(t, db, "reset-schema@test.com", "Password123", "ResetSchema", false)
	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("expected one user before reset: count=%d err=%v", count, err)
	}
	if err := db.ResetPublicSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := db.Users.Count(ctx); err != nil || count != 0 {
		t.Fatalf("reset should remove users: count=%d err=%v", count, err)
	}
	if siteName, err := db.Settings.Get(ctx, "site_name", ""); err != nil || siteName != "皮肤站" {
		t.Fatalf("reset should restore default settings: site_name=%q err=%v", siteName, err)
	}
}

func TestInitSQLContainsExpectedConstraintsAndIndexes(t *testing.T) {
	required := []string{
		"email TEXT UNIQUE NOT NULL",
		"name TEXT UNIQUE NOT NULL",
		"PRIMARY KEY(user_id, hash, texture_type)",
		"PRIMARY KEY(skin_hash, texture_type)",
		"UNIQUE(username, endpoint_id)",
		"idx_profiles_user_id",
		"idx_site_refresh_expires",
		"CREATE TABLE IF NOT EXISTS oauth_authorization_codes",
		"CREATE TABLE IF NOT EXISTS oauth_authorization_code_permissions",
		"CREATE TABLE IF NOT EXISTS oauth_refresh_tokens",
		"idx_oauth_authorization_codes_client_user",
		"idx_oauth_refresh_tokens_user_client",
		"client_type TEXT NOT NULL DEFAULT 'confidential'",
		"secret_hash TEXT NOT NULL DEFAULT ''",
		"authorization_status TEXT NOT NULL DEFAULT 'active'",
		"CHECK(authorization_status IN ('active', 'reauthorization_required'))",
		"last_refresh_at BIGINT",
		"last_refresh_error_at BIGINT",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range required {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
	}
}

func TestInitSQLUpgradesLegacyIdentityProviderAdapterConstraintExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE identity_providers DROP CONSTRAINT identity_providers_adapter_check`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE identity_providers ADD CONSTRAINT identity_providers_adapter_check CHECK (adapter IN ('generic_oidc', 'microsoft'))`); err != nil {
		t.Fatal(err)
	}
	insertProbe := func(id, adapter string) error {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO identity_providers (id,name,issuer_url,authorization_endpoint,token_endpoint,userinfo_endpoint,jwks_uri,client_id,scopes,adapter,created_at,updated_at)
			VALUES ($1,'Probe','https://probe.example','https://probe.example/authorize','https://probe.example/token','','','client',ARRAY['openid']::text[],$2,1,1)
		`, id, adapter)
		return err
	}
	if err := insertProbe("adapter-probe-legacy", "generic_oidc"); err != nil {
		t.Fatalf("legacy adapter must stay valid before the upgrade: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM identity_providers WHERE id='adapter-probe-legacy'`); err != nil {
		t.Fatal(err)
	}
	if err := insertProbe("adapter-probe-qq", "qq"); err == nil {
		t.Fatal("pre-upgrade constraint must reject the qq adapter")
	}
	if err := db.Init(ctx); err != nil {
		t.Fatalf("re-init must be idempotent and upgrade the constraint: %v", err)
	}
	if err := insertProbe("adapter-probe-qq", "qq"); err != nil {
		t.Fatalf("upgraded constraint must accept the qq adapter: %v", err)
	}
	if err := insertProbe("adapter-probe-wechat", "wechat"); err == nil {
		t.Fatal("upgraded constraint must still reject unknown adapters")
	} else if !strings.Contains(err.Error(), "identity_providers_adapter_check") {
		t.Fatalf("unknown adapter failure must come from the adapter check: %v", err)
	}
	for _, id := range []string{"adapter-probe-legacy", "adapter-probe-qq", "adapter-probe-wechat"} {
		var count int
		if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM identity_providers WHERE id=$1`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		wantLegacy := 0
		if id == "adapter-probe-qq" {
			wantLegacy = 1
		}
		if count != wantLegacy {
			t.Fatalf("probe %q rows=%d want=%d", id, count, wantLegacy)
		}
	}
}
