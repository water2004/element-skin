package database_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
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
		"('site_name', '皮肤站')",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range sqlFragments {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
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
		"oauth_device_codes", "oauth_device_code_permissions",
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
