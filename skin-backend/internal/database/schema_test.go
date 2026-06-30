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

func TestInitSQLExecutesSuccessfullyAgainstRealDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	expectedTables := []string{
		"users", "profiles", "site_refresh_tokens", "invites", "settings",
		"user_textures", "skin_library", "fallback_endpoints", "whitelisted_users",
		"verification_codes", "homepage_media", "notices", "notice_receipts",
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

func TestInitMigratesLegacyAdminColumnsToPermissionRolesAndDropsThem(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		TRUNCATE users CASCADE;
		ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE;
		ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT FALSE;
		INSERT INTO users (id,email,password,is_admin,is_super_admin,display_name,created_at) VALUES
			('z-user','z@test.com','pw',FALSE,FALSE,'Zed',300),
			('a-admin','a@test.com','pw',TRUE,FALSE,'AdminA',100),
			('b-admin','b@test.com','pw',TRUE,TRUE,'AdminB',200);
	`); err != nil {
		t.Fatal(err)
	}

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}

	var adminCount, superCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM subject_roles WHERE role_id='admin'),
			(SELECT COUNT(*) FROM subject_roles WHERE role_id='super_admin')
	`).Scan(&adminCount, &superCount); err != nil {
		t.Fatal(err)
	}
	if adminCount != 2 || superCount != 1 {
		t.Fatalf("legacy role migration counts: admin=%d super=%d; want 2 and 1", adminCount, superCount)
	}
	var superUserID string
	if err := db.Pool.QueryRow(ctx, `
		SELECT ps.user_id
		FROM subject_roles sr
		JOIN permission_subjects ps ON ps.id=sr.subject_id
		WHERE sr.role_id='super_admin'
	`).Scan(&superUserID); err != nil {
		t.Fatal(err)
	}
	if superUserID != "b-admin" {
		t.Fatalf("legacy super admin should be preserved exactly, got %q", superUserID)
	}
	for _, column := range []string{"is_admin", "is_super_admin"} {
		var exists bool
		if err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema='public' AND table_name='users' AND column_name=$1
			)
		`, column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("legacy column %s should be dropped after migration", column)
		}
	}
}

func TestInitPromotesOldestAdminOrFirstUserWhenNoLegacySuperAdminExists(t *testing.T) {
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
	var superUserID string
	if err := db.Pool.QueryRow(ctx, `
		SELECT ps.user_id
		FROM subject_roles sr
		JOIN permission_subjects ps ON ps.id=sr.subject_id
		WHERE sr.role_id='super_admin'
	`).Scan(&superUserID); err != nil {
		t.Fatal(err)
	}
	if superUserID != "a-admin" {
		t.Fatalf("oldest legacy admin should become super admin, got %q", superUserID)
	}

	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM subject_roles;
		TRUNCATE users CASCADE;
		INSERT INTO users (id,email,password,display_name,created_at) VALUES
			('z-user','z@test.com','pw','Zed',300),
			('a-user','a@test.com','pw','UserA',100);
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT ps.user_id
		FROM subject_roles sr
		JOIN permission_subjects ps ON ps.id=sr.subject_id
		WHERE sr.role_id='super_admin'
	`).Scan(&superUserID); err != nil {
		t.Fatal(err)
	}
	if superUserID != "a-user" {
		t.Fatalf("oldest user should become super admin when no admin exists, got %q", superUserID)
	}
}
