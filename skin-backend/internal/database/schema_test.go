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
	for _, fragment := range []string{
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
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_official_profile_bindings_remote_uuid",
		"('site_name', '皮肤站')",
		"ON CONFLICT (key) DO NOTHING",
	} {
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
	if err := db.Pool.QueryRow(ctx, `SELECT status,revoked_at FROM delegated_permission_grants WHERE id=$1`, oldGrant.ID).Scan(&oldStatus, &oldRevokedAt); err != nil {
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
	if oldStatus != "revoked" || oldRevokedAt == nil || *oldRevokedAt <= 0 || refreshRevokedAt == nil ||
		*refreshRevokedAt != *oldRevokedAt || oldCodeCount != 0 || newStatus != "active" || activeCount != 1 {
		t.Fatalf("grant migration mismatch: old_status=%q old_revoked_at=%v refresh_revoked_at=%v old_codes=%d new_status=%q active=%d", oldStatus, oldRevokedAt, refreshRevokedAt, oldCodeCount, newStatus, activeCount)
	}
	third := newGrant
	third.ID = "schema-third-active-grant"
	if err := db.OAuth.CreateGrant(ctx, third, []int64{permissionID}); err == nil {
		t.Fatal("restored unique index should reject another active grant")
	}
}

func TestInitMigratesPublishedV302OIDCStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE delegated_permission_grants DROP COLUMN oidc_scopes CASCADE;
		ALTER TABLE oauth_authorization_codes DROP COLUMN oidc_scopes, DROP COLUMN nonce;
		ALTER TABLE oauth_refresh_tokens DROP COLUMN oidc_scopes;
		INSERT INTO permission_resources (id,code,description,created_at)
		VALUES (65000,'microsoft_import','v3 resource',1);
		INSERT INTO permission_actions (id,code,description,created_at)
		VALUES (65000,'v3_import','v3 action',1);
		INSERT INTO permission_scopes (id,code,resolver_key,description,created_at)
		VALUES (65000,'v3_owned','owned','v3 scope',1);
		INSERT INTO permissions (id,code,resource_id,action_id,scope_id,description,created_at)
		VALUES (279177134145000,'microsoft_import.create.owned',65000,65000,65000,'v3 permission',1);
	`); err != nil {
		t.Fatal(err)
	}

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}

	wantColumns := map[string][]string{
		"delegated_permission_grants": {"oidc_scopes"},
		"oauth_authorization_codes":   {"nonce", "oidc_scopes"},
		"oauth_refresh_tokens":        {"oidc_scopes"},
	}
	for table, expected := range wantColumns {
		rows, err := db.Pool.Query(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1
				AND column_name=ANY($2::TEXT[])
			ORDER BY column_name
		`, table, expected)
		if err != nil {
			t.Fatal(err)
		}
		var actual []string
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			actual = append(actual, column)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if strings.Join(actual, ",") != strings.Join(expected, ",") {
			t.Fatalf("%s migrated columns=%v, want %v", table, actual, expected)
		}
	}

	var oldPermissionCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permissions WHERE code LIKE 'microsoft_import.%'`).Scan(&oldPermissionCount); err != nil {
		t.Fatal(err)
	}
	if oldPermissionCount != 0 {
		t.Fatalf("migrated Microsoft permissions=%d, want 0", oldPermissionCount)
	}
}

func TestInitPreservesV302MicrosoftSettingsForTransactionalServiceMigration(t *testing.T) {
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
			t.Fatalf("v3 setting %s changed during schema init: value=%q want=%q err=%v", key, value, expected, err)
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
		"permission_subjects", "permission_resources", "permission_actions", "permission_scopes", "permissions",
		"roles", "role_permissions", "subject_roles", "subject_permission_overrides", "session_permission_policies",
		"identity_providers", "external_identities", "external_identity_credentials", "official_profile_bindings",
		"oauth_device_codes", "oauth_device_code_permissions", "webhook_endpoints", "webhook_endpoint_events",
		"webhook_events", "webhook_deliveries",
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
