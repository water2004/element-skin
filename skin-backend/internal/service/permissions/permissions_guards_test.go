package permissions_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
)

func TestPermissionServiceRejectsUnauthorizedAndInvalidOverridePathsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-errors@test.com", "Password123", "AdminPermsErrors", true)
	target := testutil.CreateUser(t, db, "target-perms-errors@test.com", "Password123", "TargetPermsErrors", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	reader := actorWithPermissions(adminUser.ID, "permission.read.any")

	if _, err := svc.UserPermissions(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("read without permission error mismatch: %#v", err)
	}
	if _, err := svc.UserPermissions(ctx, reader, "missing-user"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("missing user read error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "missing.permission.any", "allow"); !httpErrorIs(err, http.StatusNotFound, "permission.resolve.not_found") {
		t.Fatalf("missing permission set error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "notice.create.any", "maybe"); !httpErrorIs(err, http.StatusBadRequest, "permission_override.configure.invalid") {
		t.Fatalf("invalid effect error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "notice.create.any", "allow"); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("grant without permission error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, reader, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusNotFound, "permission_override.resolve.not_found") {
		t.Fatalf("clear missing override error mismatch: %#v", err)
	}

	writer := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := svc.SetUserPermissionOverride(ctx, writer, "missing-user", "notice.create.any", "allow"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("set missing user error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, writer, "missing-user", "notice.create.any"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("clear missing user error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, writer, target.ID, "missing.permission.any"); !httpErrorIs(err, http.StatusNotFound, "permission.resolve.not_found") {
		t.Fatalf("clear missing permission error mismatch: %#v", err)
	}
}

func TestPermissionServiceProtectsProtectedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-protected@test.com", "Password123", "AdminProtected", true)
	target := testutil.CreateUser(t, db, "target-protected@test.com", "Password123", "TargetProtected", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	grantOnly := actorWithPermissions(adminUser.ID, "permission.grant.any")

	err := svc.SetUserPermissionOverride(ctx, grantOnly, target.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected_permission.transfer.required") {
		t.Fatalf("protected management grant error mismatch: %#v", err)
	}
	selfManager := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission_protected.manage.any")
	err = svc.SetUserPermissionOverride(ctx, selfManager, adminUser.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected_permission.transfer.required") {
		t.Fatalf("self protected management grant error mismatch: %#v", err)
	}
	manager := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any", "permission_protected.manage.any")
	err = svc.SetUserPermissionOverride(ctx, manager, target.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected_permission.transfer.required") {
		t.Fatalf("manager protected management grant error mismatch: %#v", err)
	}
	if overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID); err != nil || len(overrides) != 0 {
		t.Fatalf("rejected protected management grants must not mutate overrides: overrides=%#v err=%v", overrides, err)
	}
	if err := svc.SetUserPermissionOverride(ctx, grantOnly, target.ID, "cache.invalidate.system", "allow"); !httpErrorIs(err, http.StatusForbidden, "protected_permission.manage.required") {
		t.Fatalf("system-scope grant without manage error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, manager, target.ID, "cache.invalidate.system", "allow"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, manager, target.ID, "cache.invalidate.system"); err != nil {
		t.Fatal(err)
	}
}
