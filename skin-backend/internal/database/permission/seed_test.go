package permission_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestSeedDefaultsPersistsCatalogExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	var permissionCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permissions`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if permissionCount != len(core.Definitions) {
		t.Fatalf("permission count mismatch: got=%d want=%d", permissionCount, len(core.Definitions))
	}
	def := core.MustDefinitionByCode("permission_protected.manage.any")
	var id int64
	var resourceID int
	var actionID int
	var scopeID int
	if err := db.Pool.QueryRow(ctx, `
		SELECT id,resource_id,action_id,scope_id
		FROM permissions
		WHERE code='permission_protected.manage.any'
	`).Scan(&id, &resourceID, &actionID, &scopeID); err != nil {
		t.Fatal(err)
	}
	if id != int64(def.ID) || resourceID != int(def.Resource.ID) || actionID != int(def.Action.ID) || scopeID != int(def.Scope.ID) {
		t.Fatalf("seeded permission mismatch: id=%#x/%#x resource=%d/%d action=%d/%d scope=%d/%d",
			id, int64(def.ID), resourceID, def.Resource.ID, actionID, def.Action.ID, scopeID, def.Scope.ID)
	}
	var roleCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE system_role=TRUE`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != len(core.Roles) {
		t.Fatalf("system role count mismatch: got=%d want=%d", roleCount, len(core.Roles))
	}
}

func TestSeedPreservesExistingAdminRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "permission-admin@test.com", "pw", "PermissionAdmin", true)
	protectedUser := testutil.CreateUser(t, db, "permission-protected@test.com", "pw", "PermissionProtected", true, true)
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	adminBits, err := db.Permissions.EffectivePermissionsForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(adminBits, "notice.create.any") {
		t.Fatal("admin should retain notice creation permission")
	}
	if has(adminBits, "permission_protected.manage.any") {
		t.Fatal("admin must not manage protected permission subjects")
	}
	protected, err := db.Permissions.UserIsProtected(ctx, protectedUser.ID)
	if err != nil || !protected {
		t.Fatalf("existing protected subject should remain protected: protected=%v err=%v", protected, err)
	}
}

func TestSeedDefaultsFirstRegisteredUserBecomesProtectedSubject(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	user := testutil.CreateUser(t, db, "first-user@test.com", "pw", "FirstUser", false)
	if _, err := db.Pool.Exec(ctx, `UPDATE permission_subjects SET protected=FALSE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permission_subjects WHERE protected=TRUE`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("SeedDefaults should ensure exactly one protected subject after removal: got=%d", count)
	}
	protected, err := db.Permissions.UserIsProtected(ctx, user.ID)
	if err != nil || !protected {
		t.Fatalf("the only user should become protected when none exists: protected=%v err=%v", protected, err)
	}
}

func TestSeedDefaultsFailsWhenCatalogTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_resources CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenSessionPoliciesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenSubjectRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE subject_roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionActionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_actions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenRolePermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE role_permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionSubjectsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_subjects CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}
