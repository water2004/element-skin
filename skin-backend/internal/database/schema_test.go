package database_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestInitSQLContainsExpectedTablesConstraintsIndexesAndSeeds(t *testing.T) {
	sqlFragments := []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE TABLE IF NOT EXISTS profiles",
		"CREATE TABLE IF NOT EXISTS email_suffix_policy",
		"CREATE TABLE IF NOT EXISTS email_suffix_rules",
		"email TEXT UNIQUE NOT NULL",
		"name TEXT UNIQUE NOT NULL",
		"PRIMARY KEY(user_id, hash, texture_type)",
		"PRIMARY KEY(skin_hash, texture_type)",
		"UNIQUE(username, endpoint_id)",
		"idx_profiles_user_id",
		"idx_site_refresh_expires",
		"idx_delegated_permission_grants_active_user_client",
		"ALTER TABLE identity_providers DROP COLUMN IF EXISTS registration_enabled",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_official_profile_bindings_remote_uuid",
		"('site_name', '皮肤站')",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range sqlFragments {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
	}
}

func TestInitConsolidatesDuplicateActiveOAuthGrantsAndInvalidatesOldCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "schema-duplicate-grants@test.com", "Password123", "SchemaDuplicateGrants", false)
	permissionID := int64(permission.MustDefinitionByCode("account.read.self").ID)
	client := model.OAuthClient{
		ID:          "schema-duplicate-grant-client",
		OwnerUserID: user.ID,
		Name:        "Schema duplicate grant client",
		RedirectURI: "https://schema-duplicate-grants.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, []int64{permissionID}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DROP INDEX idx_delegated_permission_grants_active_user_client`); err != nil {
		t.Fatal(err)
	}
	oldGrant := model.OAuthGrant{ID: "schema-old-active-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 1100}
	newGrant := model.OAuthGrant{ID: "schema-new-active-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 1200}
	for _, grant := range []model.OAuthGrant{oldGrant, newGrant} {
		if err := db.OAuth.CreateGrant(ctx, grant, []int64{permissionID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{TokenHash: "schema-old-refresh", ClientID: client.ID, UserID: user.ID, GrantID: oldGrant.ID, ExpiresAt: 9999, CreatedAt: 1300}); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
		CodeHash:            "schema-old-code",
		ClientID:            client.ID,
		UserID:              user.ID,
		GrantID:             oldGrant.ID,
		RedirectURI:         client.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           9999,
		CreatedAt:           1300,
	}, []int64{permissionID}); err != nil {
		t.Fatal(err)
	}

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	var oldStatus, newStatus string
	var oldRevokedAt, refreshRevokedAt *int64
	if err := db.Pool.QueryRow(ctx, `SELECT status, revoked_at FROM delegated_permission_grants WHERE id=$1`, oldGrant.ID).Scan(&oldStatus, &oldRevokedAt); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT status FROM delegated_permission_grants WHERE id=$1`, newGrant.ID).Scan(&newStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT revoked_at FROM oauth_refresh_tokens WHERE token_hash='schema-old-refresh'`).Scan(&refreshRevokedAt); err != nil {
		t.Fatal(err)
	}
	var oldCodeCount, activeCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_authorization_codes WHERE code_hash='schema-old-code'`).Scan(&oldCodeCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM delegated_permission_grants WHERE user_id=$1 AND client_id=$2 AND status='active'`, user.ID, client.ID).Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "revoked" || oldRevokedAt == nil || *oldRevokedAt <= 0 || refreshRevokedAt == nil || *refreshRevokedAt != *oldRevokedAt || oldCodeCount != 0 || newStatus != "active" || activeCount != 1 {
		t.Fatalf("grant migration mismatch: old_status=%q old_revoked_at=%v refresh_revoked_at=%v old_codes=%d new_status=%q active=%d", oldStatus, oldRevokedAt, refreshRevokedAt, oldCodeCount, newStatus, activeCount)
	}
	third := newGrant
	third.ID = "schema-third-active-grant"
	if err := db.OAuth.CreateGrant(ctx, third, []int64{permissionID}); err == nil {
		t.Fatal("restored unique index should reject another active grant")
	}
}

func TestInitSQLContainsOnlyVersion241MigrationPaths(t *testing.T) {
	for _, fragment := range []string{
		"ALTER TABLE skin_library DROP CONSTRAINT IF EXISTS skin_library_pkey",
		"ALTER TABLE skin_library ADD CONSTRAINT skin_library_pkey PRIMARY KEY (skin_hash, texture_type)",
		"ALTER TABLE skin_library ADD COLUMN IF NOT EXISTS usage_count",
		"DROP TABLE IF EXISTS sessions",
		"DROP TABLE IF EXISTS tokens",
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at",
		"UPDATE users SET created_at = 0 WHERE created_at IS NULL",
		"UPDATE skin_library sl SET usage_count",
		"ALTER TABLE fallback_endpoints DROP COLUMN skin_domains",
	} {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing version 2.4.1 migration fragment %q", fragment)
		}
	}
	for _, fragment := range []string{
		"ALTER TABLE permissions DROP COLUMN IF EXISTS bit_index",
		"ALTER TABLE homepage_media DROP COLUMN IF EXISTS config",
		"ALTER TABLE delegated_clients ADD COLUMN IF NOT EXISTS",
		"ALTER TABLE oauth_device_codes ADD COLUMN IF NOT EXISTS",
		"ALTER TABLE permission_subjects ADD COLUMN IF NOT EXISTS protected",
		"DELETE FROM settings WHERE key IN ('fallback_services', 'easter_eggs_enabled')",
		"DELETE FROM settings WHERE key IN ('microsoft_client_id', 'microsoft_client_secret', 'microsoft_redirect_uri')",
	} {
		if strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL contains development-only migration fragment %q", fragment)
		}
	}
}

func TestInitPreservesLegacyMicrosoftSettingsForTransactionalServiceMigration(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	want := map[string]string{
		"microsoft_client_id":     "legacy-client",
		"microsoft_client_secret": "legacy-secret",
		"microsoft_redirect_uri":  "https://old.example/callback",
	}
	for key, value := range want {
		if err := db.Settings.Set(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for key, expected := range want {
		value, err := db.Settings.Get(ctx, key, "missing")
		if err != nil || value != expected {
			t.Fatalf("legacy setting %s changed during schema init: value=%q want=%q err=%v", key, value, expected, err)
		}
	}
}

func TestInitSQLExecutesSuccessfullyAgainstRealDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	expectedTables := []string{
		"users", "profiles", "site_refresh_tokens", "invites", "settings", "email_suffix_policy", "email_suffix_rules",
		"user_textures", "skin_library", "fallback_endpoints", "fallback_skin_domains", "whitelisted_users",
		"verification_codes", "homepage_media", "enabled_easter_eggs", "notices", "notice_receipts",
		"permission_subjects", "permission_resources", "permission_actions",
		"permission_scopes", "permissions", "roles", "role_permissions",
		"subject_roles", "subject_permission_overrides",
		"session_permission_policies",
		"identity_providers", "external_identities", "external_identity_credentials", "official_profile_bindings",
		"oauth_device_codes", "oauth_device_code_permissions",
		"webhook_endpoints", "webhook_endpoint_events", "webhook_events", "webhook_deliveries",
	}
	for _, table := range expectedTables {
		var exists bool
		if err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema='public' AND table_name=$1
			)
		`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("InitSQL should create table %q", table)
		}
	}
}

func TestInitRemovesRegistrationSwitchAndConsolidatesOfficialUUIDBindingsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "schema-official-binding@test.com", "Password123", "SchemaOfficial", false)
	provider := model.IdentityProvider{
		ID: "schema-identity-provider", Name: "Microsoft", IssuerURL: "https://login.example",
		AuthorizationEndpoint: "https://login.example/authorize", TokenEndpoint: "https://login.example/token",
		JWKSURI: "https://login.example/jwks", ClientID: "client", Adapter: "microsoft",
		Enabled: true, LoginEnabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	identity := model.ExternalIdentity{
		ID: "schema-external-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "schema-subject", CreatedAt: 2, UpdatedAt: 2,
	}
	if err := db.Identities.CreateIdentity(ctx, identity, model.ExternalIdentityCredential{IdentityID: identity.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	remoteUUID := "0123456789abcdef0123456789abcdef"
	canonical := testutil.CreateProfile(t, db, user.ID, remoteUUID, "CanonicalOfficial")
	legacy := testutil.CreateProfile(t, db, user.ID, "schema-legacy-official", "LegacyOfficial")
	if _, err := db.Pool.Exec(ctx, `DROP INDEX idx_official_profile_bindings_remote_uuid`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE identity_providers ADD COLUMN registration_enabled BOOLEAN NOT NULL DEFAULT TRUE
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES
			('schema-legacy-binding',$1,$2,$3,'LegacyOfficial',10,10),
			('schema-canonical-binding',$1,$3,$3,'CanonicalOfficial',20,20)
	`, identity.ID, legacy.ID, canonical.ID); err != nil {
		t.Fatal(err)
	}

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	var registrationColumnExists bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name='identity_providers'
				AND column_name='registration_enabled'
		)
	`).Scan(&registrationColumnExists); err != nil {
		t.Fatal(err)
	}
	var bindingID, profileID string
	var bindingCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT MIN(id),MIN(profile_id),COUNT(*)
		FROM official_profile_bindings
		WHERE remote_uuid=$1
	`, remoteUUID).Scan(&bindingID, &profileID, &bindingCount); err != nil {
		t.Fatal(err)
	}
	if registrationColumnExists || bindingCount != 1 || bindingID != "schema-canonical-binding" || profileID != canonical.ID {
		t.Fatalf("identity migration mismatch: registration_column=%v count=%d binding=%q profile=%q", registrationColumnExists, bindingCount, bindingID, profileID)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,created_at,updated_at)
		VALUES ('schema-duplicate-binding',$1,$2,$3,'Duplicate',30,30)
	`, identity.ID, legacy.ID, remoteUUID); err == nil {
		t.Fatal("restored remote UUID unique index should reject another binding")
	}
}

func TestInitMigratesVersion241AdminColumnToPermissionRolesAndDropsIt(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		TRUNCATE users CASCADE;
		ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE;
		INSERT INTO users (id,email,password,is_admin,display_name,created_at) VALUES
			('z-user','z@test.com','pw',FALSE,'Zed',300),
			('a-admin','a@test.com','pw',TRUE,'AdminA',100),
			('b-admin','b@test.com','pw',TRUE,'AdminB',200);
	`); err != nil {
		t.Fatal(err)
	}

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}

	var adminCount, protectedCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM subject_roles WHERE role_id='admin'),
			(SELECT COUNT(*) FROM permission_subjects WHERE protected=TRUE)
	`).Scan(&adminCount, &protectedCount); err != nil {
		t.Fatal(err)
	}
	if adminCount != 2 || protectedCount != 1 {
		t.Fatalf("2.4.1 role migration counts: admin=%d protected=%d; want 2 and 1", adminCount, protectedCount)
	}
	var protectedUserID string
	if err := db.Pool.QueryRow(ctx, `
		SELECT user_id
		FROM permission_subjects
		WHERE protected=TRUE
	`).Scan(&protectedUserID); err != nil {
		t.Fatal(err)
	}
	if protectedUserID != "a-admin" {
		t.Fatalf("oldest 2.4.1 admin should become protected subject, got %q", protectedUserID)
	}
	var exists bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema='public' AND table_name='users' AND column_name='is_admin'
		)
	`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("2.4.1 is_admin column should be dropped after migration")
	}
}

func TestInitMigratesDelimitedFallbackDomainsToStructuredRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE fallback_endpoints ADD COLUMN skin_domains TEXT DEFAULT '';
		INSERT INTO fallback_endpoints (
			priority,session_url,account_url,services_url,cache_ttl,skin_domains,
			enable_profile,enable_hasjoined,enable_whitelist,note
		) VALUES (1,'session','account','services',60,' first.example, second.example, first.example ',TRUE,TRUE,FALSE,'migration');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	domains, err := db.Fallbacks.CollectSkinDomains(ctx)
	if err != nil || strings.Join(domains, ",") != "first.example,second.example" {
		t.Fatalf("migrated fallback domains=%#v err=%v, want exact ordered unique rows", domains, err)
	}
	var legacyColumnExists bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name='fallback_endpoints' AND column_name='skin_domains'
		)
	`).Scan(&legacyColumnExists); err != nil {
		t.Fatal(err)
	}
	if legacyColumnExists {
		t.Fatal("legacy fallback_endpoints.skin_domains column should be removed")
	}
}

func TestInitPromotesOldestUserWhenVersion241HasNoAdmin(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		TRUNCATE users CASCADE;
		ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE;
		INSERT INTO users (id,email,password,is_admin,display_name,created_at) VALUES
			('z-user','z@test.com','pw',FALSE,'Zed',300),
			('a-user','a@test.com','pw',FALSE,'UserA',100);
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	var protectedUserID string
	if err := db.Pool.QueryRow(ctx, `
		SELECT user_id
		FROM permission_subjects
		WHERE protected=TRUE
	`).Scan(&protectedUserID); err != nil {
		t.Fatal(err)
	}
	if protectedUserID != "a-user" {
		t.Fatalf("oldest 2.4.1 user should become protected subject when no admin exists, got %q", protectedUserID)
	}
}

func TestInitMigratesVersion241YggdrasilTablesAndSkinLibraryPrimaryKey(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		CREATE TABLE tokens (
			access_token TEXT PRIMARY KEY,
			client_token TEXT NOT NULL,
			user_id TEXT NOT NULL,
			profile_id TEXT,
			created_at BIGINT NOT NULL
		);
		CREATE TABLE sessions (
			server_id TEXT PRIMARY KEY,
			access_token TEXT NOT NULL,
			ip TEXT,
			created_at BIGINT NOT NULL
		);
		DROP INDEX IF EXISTS idx_skin_library_public_usage_created_hash;
		ALTER TABLE skin_library DROP COLUMN usage_count;
		ALTER TABLE skin_library DROP CONSTRAINT skin_library_pkey;
		ALTER TABLE skin_library ADD CONSTRAINT skin_library_pkey PRIMARY KEY (skin_hash);
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tokens", "sessions"} {
		var exists bool
		if err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema='public' AND table_name=$1
			)
		`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("2.4.1 table %s should be removed after migration", table)
		}
	}
	var usageCountExists bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema='public' AND table_name='skin_library' AND column_name='usage_count'
		)
	`).Scan(&usageCountExists); err != nil {
		t.Fatal(err)
	}
	if !usageCountExists {
		t.Fatal("skin_library usage_count should be added during 2.4.1 migration")
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=ANY(i.indkey)
		WHERE i.indrelid='skin_library'::regclass AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, column)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	if got := strings.Join(columns, ","); got != "skin_hash,texture_type" {
		t.Fatalf("skin_library primary key mismatch: got %q want %q", got, "skin_hash,texture_type")
	}
}
