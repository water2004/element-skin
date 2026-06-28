package permission_test

import (
	"context"
	"testing"
	"time"

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
	var bitIndex int
	var resourceID int
	var actionID int
	var scopeID int
	if err := db.Pool.QueryRow(ctx, `
		SELECT id,bit_index,resource_id,action_id,scope_id
		FROM permissions
		WHERE code='permission_protected.manage.any'
	`).Scan(&id, &bitIndex, &resourceID, &actionID, &scopeID); err != nil {
		t.Fatal(err)
	}
	if id != int64(def.ID) || bitIndex != def.BitIndex || resourceID != int(def.Resource.ID) || actionID != int(def.Action.ID) || scopeID != int(def.Scope.ID) {
		t.Fatalf("seeded permission mismatch: id=%#x/%#x bit=%d/%d resource=%d/%d action=%d/%d scope=%d/%d",
			id, int64(def.ID), bitIndex, def.BitIndex, resourceID, def.Resource.ID, actionID, def.Action.ID, scopeID, def.Scope.ID)
	}
	var roleCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE system_role=TRUE`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != len(core.Roles) {
		t.Fatalf("system role count mismatch: got=%d want=%d", roleCount, len(core.Roles))
	}
}

func TestEffectivePermissionsIncludeDefaultUserRoleAndExactOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-user@test.com", "pw", "PermissionUser", false)
	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(before, "texture.update_visibility.owned") {
		t.Fatal("default user should be allowed to update visibility of owned textures")
	}
	if has(before, "permission_protected.manage.any") {
		t.Fatal("default user must not manage protected permission subjects")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, core.MustDefinitionByCode("texture.update_visibility.owned"), "deny", ""); err != nil {
		t.Fatal(err)
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(after, "texture.update_visibility.owned") {
		t.Fatal("deny override should remove texture.update_visibility.owned exactly")
	}
	if !has(after, "texture.update_metadata.owned") {
		t.Fatal("deny override should not remove neighboring texture.update_metadata.owned")
	}
}

func TestSeedMigratesExistingAdminFlagsToRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "permission-admin@test.com", "pw", "PermissionAdmin", true)
	super := testutil.CreateUser(t, db, "permission-super@test.com", "pw", "PermissionSuper", true, true)
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	adminBits, err := db.Permissions.EffectivePermissionsForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(adminBits, "notice.create.any") {
		t.Fatal("migrated admin should create notices")
	}
	if has(adminBits, "permission_protected.manage.any") {
		t.Fatal("migrated admin must not manage protected permission subjects")
	}
	superBits, err := db.Permissions.EffectivePermissionsForUser(ctx, super.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(superBits, "permission_protected.manage.any") {
		t.Fatal("migrated super admin should manage protected permission subjects")
	}
	if has(superBits, "cache.invalidate.system") {
		t.Fatal("super admin should not receive system-scope permissions")
	}
}

func TestSessionPolicyAndBanNarrowPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-ygg@test.com", "pw", "PermissionYgg", false)
	unbanned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindYggdrasil,
		Entrypoint:  core.EntrypointYggdrasil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(unbanned, "yggdrasil_server.join.bound_profile") || !has(unbanned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("yggdrasil session should include join and hasjoined before ban policy")
	}
	if has(unbanned, "account.read.self") {
		t.Fatal("yggdrasil session should not include dashboard account permissions")
	}
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, bannedUntil, user.ID); err != nil {
		t.Fatal(err)
	}
	banned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if has(banned, "yggdrasil_server.join.bound_profile") {
		t.Fatal("ban policy should revoke only yggdrasil_server.join.bound_profile")
	}
	if !has(banned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("ban policy should keep yggdrasil_server.hasjoined.bound_profile")
	}
}

func TestDelegationPolicyIntersectsSubjectClientAndGrantExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-delegated@test.com", "pw", "PermissionDelegated", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	allowedByUser := core.MustDefinitionByCode("texture.update_visibility.owned")
	notAllowedByUser := core.MustDefinitionByCode("account.ban.any")
	clientOnlyMissing := core.MustDefinitionByCode("profile.create.owned")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_clients (id,owner_user_id,name,status,created_at,updated_at)
		VALUES ('client-1',$1,'Client','active',$2,$2)
	`, user.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_client_permissions (client_id,permission_id,created_at)
		VALUES ('client-1',$1,$3),('client-1',$2,$3)
	`, int64(allowedByUser.ID), int64(notAllowedByUser.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id,user_id,subject_id,client_id,status,created_at)
		VALUES ('grant-1',$1,$2,'client-1','active',$3)
	`, user.ID, permissiondb.SubjectIDForUser(user.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_grant_permissions (grant_id,permission_id,created_at)
		VALUES ('grant-1',$1,$4),('grant-1',$2,$4),('grant-1',$3,$4)
	`, int64(allowedByUser.ID), int64(notAllowedByUser.ID), int64(clientOnlyMissing.ID), now); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:       core.SessionKindWeb,
		Entrypoint:        core.EntrypointDashboard,
		DelegatedClientID: "client-1",
		DelegatedGrantID:  "grant-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, allowedByUser.Code) {
		t.Fatal("delegated permissions should keep permission allowed by user, client and grant")
	}
	if has(bits, notAllowedByUser.Code) {
		t.Fatal("delegated permissions must remove permission missing from subject effective permissions")
	}
	if has(bits, clientOnlyMissing.Code) {
		t.Fatal("delegated permissions must remove permission missing from client allow list")
	}
}

func TestGrantAndRevokeRoleExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "role-grant-revoke@test.com", "pw", "RoleGrantRevoke", false)
	adminID := testutil.CreateUser(t, db, "role-admin@test.com", "pw", "RoleAdmin", true).ID

	hasModerator, err := db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || hasModerator {
		t.Fatalf("new user should not have moderator role: has=%v err=%v", hasModerator, err)
	}
	if err := db.Permissions.GrantRole(ctx, user.ID, core.RoleModerator, permissiondb.SubjectIDForUser(adminID)); err != nil {
		t.Fatal(err)
	}
	hasModerator, err = db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || !hasModerator {
		t.Fatalf("user should have moderator role after grant: has=%v err=%v", hasModerator, err)
	}
	roles, err := db.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range roles {
		if r == core.RoleModerator {
			found = true
		}
	}
	if !found {
		t.Fatalf("RoleIDsForUser should include moderator: %#v", roles)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, "texture.review.assigned") {
		t.Fatal("granted moderator should have texture.review.assigned")
	}

	revoked, err := db.Permissions.RevokeRole(ctx, user.ID, core.RoleModerator)
	if err != nil || !revoked {
		t.Fatalf("RevokeRole should return revoked=true: revoked=%v err=%v", revoked, err)
	}
	hasModerator, err = db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || hasModerator {
		t.Fatalf("role should be removed after revoke: has=%v err=%v", hasModerator, err)
	}
	revokedAgain, err := db.Permissions.RevokeRole(ctx, user.ID, core.RoleModerator)
	if err != nil || revokedAgain {
		t.Fatalf("revoking missing role should return revoked=false: revoked=%v err=%v", revokedAgain, err)
	}
}

func TestUserHasProtectedRoleExact(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "protected-role@test.com", "pw", "ProtectedRole", false)

	hasProtected, err := db.Permissions.UserHasProtectedRole(ctx, user.ID)
	if err != nil || hasProtected {
		t.Fatalf("normal user should not have protected role: has=%v err=%v", hasProtected, err)
	}
	if err := db.Permissions.GrantRole(ctx, user.ID, core.RoleSuperAdmin, ""); err != nil {
		t.Fatal(err)
	}
	hasProtected, err = db.Permissions.UserHasProtectedRole(ctx, user.ID)
	if err != nil || !hasProtected {
		t.Fatalf("super admin should have protected role: has=%v err=%v", hasProtected, err)
	}
}

func TestSetSubjectPermissionOverrideAllowEffect(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-allow@test.com", "pw", "OverrideAllow", false)
	admin := testutil.CreateUser(t, db, "override-admin@test.com", "pw", "OverrideAdmin", true)
	allowDef := core.MustDefinitionByCode("notice.create.any")
	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(before, allowDef.Code) {
		t.Fatal("normal user should not have notice.create.any before override")
	}
	grantorSubjectID := permissiondb.SubjectIDForUser(admin.ID)
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, allowDef, "allow", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(after, allowDef.Code) {
		t.Fatal("allow override should grant notice.create.any")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, allowDef, "deny", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	afterDeny, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(afterDeny, allowDef.Code) {
		t.Fatal("deny override should supersede allow override")
	}
}

func TestSetSubjectPermissionOverrideRejectsInvalidEffect(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-invalid@test.com", "pw", "OverrideInvalid", false)
	def := core.MustDefinitionByCode("notice.create.any")
	err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "invalid", "")
	if err == nil || err.Error() != "permission override effect must be allow or deny" {
		t.Fatalf("invalid override effect should be rejected exactly: %v", err)
	}
}

func TestGrantInitialSuperAdminWhenExists(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	first := testutil.CreateUser(t, db, "first-super@test.com", "pw", "FirstSuper", false)
	granted, err := db.Permissions.GrantInitialSuperAdminIfNone(ctx, first.ID)
	if err != nil || !granted {
		t.Fatalf("first user should receive super admin: granted=%v err=%v", granted, err)
	}
	second := testutil.CreateUser(t, db, "second-super@test.com", "pw", "SecondSuper", false)
	grantedAgain, err := db.Permissions.GrantInitialSuperAdminIfNone(ctx, second.ID)
	if err != nil || grantedAgain {
		t.Fatalf("second call should not grant super admin again: granted=%v err=%v", grantedAgain, err)
	}
	hasSuper, err := db.Permissions.UserHasRole(ctx, second.ID, core.RoleSuperAdmin)
	if err != nil || hasSuper {
		t.Fatalf("second user should not have super admin role: has=%v err=%v", hasSuper, err)
	}
}

func TestEnsureUserSubjectIdempotent(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ensure-subject@test.com", "pw", "EnsureSubject", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatalf("double EnsureUserSubject should be idempotent: %v", err)
	}
	roles, err := db.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	userCount := 0
	for _, r := range roles {
		if r == core.RoleUser {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("EnsureUserSubject should assign user role exactly once: roles=%#v", roles)
	}
}

func TestActorForUserExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-for-user@test.com", "pw", "ActorForUser", false)

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindWeb,
		Entrypoint:  core.EntrypointDashboard,
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.SubjectID != permissiondb.SubjectIDForUser(user.ID) {
		t.Fatalf("SubjectID=%q want=%q", actor.SubjectID, permissiondb.SubjectIDForUser(user.ID))
	}
	if actor.UserID != user.ID {
		t.Fatalf("UserID=%q want=%q", actor.UserID, user.ID)
	}
	if actor.SessionKind != core.SessionKindWeb || actor.Entrypoint != core.EntrypointDashboard {
		t.Fatalf("session fields mismatch: kind=%q entrypoint=%q", actor.SessionKind, actor.Entrypoint)
	}
	if actor.Permissions.Empty() {
		t.Fatal("actor should have non-empty permissions")
	}
	if !actor.Permissions.Has(core.MustDefinitionByCode("profile.create.owned").BitIndex) {
		t.Fatal("web actor should have profile.create.owned")
	}
	if actor.Permissions.Has(core.MustDefinitionByCode("notice.create.any").BitIndex) {
		t.Fatal("normal user should not have notice.create.any in actor")
	}
}

func TestActorForUserWithBanPolicy(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-ban@test.com", "pw", "ActorBan", false)
	if err := db.Users.Ban(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	joinBit := core.MustDefinitionByCode("yggdrasil_server.join.bound_profile").BitIndex
	if actor.Permissions.Has(joinBit) {
		t.Fatal("banned user joined via actor should have join permission cleared")
	}
	hasJoinedBit := core.MustDefinitionByCode("yggdrasil_server.hasjoined.bound_profile").BitIndex
	if !actor.Permissions.Has(hasJoinedBit) {
		t.Fatal("banned user should still have hasjoined permission")
	}
}

func TestRoleIDsForUserRejectsNonexistentUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	roles, err := db.Permissions.RoleIDsForUser(ctx, "nonexistent-user-id")
	if err == nil {
		t.Fatalf("RoleIDsForUser should reject nonexistent user: roles=%#v", roles)
	}
}

func TestEffectivePermissionsRejectsNonexistentUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	opts := permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, "nonexistent-ban-check", opts)
	if err == nil {
		t.Fatalf("EffectivePermissionsForUser should reject nonexistent user: bits=%#v", bits)
	}
}

func TestSetSubjectPermissionOverrideIdempotent(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-idempotent@test.com", "pw", "OverrideIdempotent", false)
	def := core.MustDefinitionByCode("texture.update_visibility.owned")

	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "deny", ""); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(bits, def.Code) {
		t.Fatal("first deny should remove the permission")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	bits, err = db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, def.Code) {
		t.Fatal("allow override after deny should restore the permission")
	}
}

func TestActorForUserWithDelegationFieldsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-delegation@test.com", "pw", "ActorDelegation", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	def := core.MustDefinitionByCode("texture.update_visibility.owned")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_clients (id,owner_user_id,name,status,created_at,updated_at)
		VALUES ('actor-client',$1,'ActorClient','active',$2,$2)
	`, user.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_client_permissions (client_id,permission_id,created_at)
		VALUES ('actor-client',$1,$2)
	`, int64(def.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id,user_id,subject_id,client_id,status,created_at)
		VALUES ('actor-grant',$1,$2,'actor-client','active',$3)
	`, user.ID, permissiondb.SubjectIDForUser(user.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_grant_permissions (grant_id,permission_id,created_at)
		VALUES ('actor-grant',$1,$2)
	`, int64(def.ID), now); err != nil {
		t.Fatal(err)
	}

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:       core.SessionKindWeb,
		Entrypoint:        core.EntrypointDashboard,
		DelegatedGrantID:  "actor-grant",
		DelegatedClientID: "actor-client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.DelegationID != "actor-grant" {
		t.Fatalf("DelegationID=%q want=actor-grant", actor.DelegationID)
	}
	if actor.DelegatedClientID != "actor-client" {
		t.Fatalf("DelegatedClientID=%q want=actor-client", actor.DelegatedClientID)
	}
	if !actor.Permissions.Has(def.BitIndex) {
		t.Fatal("delegated actor should have the granted permission")
	}
}

func TestSeedUserSubjectsMigratesIsAdminColumnExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	adminUser := testutil.CreateUser(t, db, "migrate-admin@test.com", "pw", "MigrateAdmin", false)
	normalUser := testutil.CreateUser(t, db, "migrate-normal@test.com", "pw", "MigrateNormal", false)

	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET is_admin=TRUE WHERE id=$1`, adminUser.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	hasAdmin, err := db.Permissions.UserHasRole(ctx, adminUser.ID, core.RoleAdmin)
	if err != nil || !hasAdmin {
		t.Fatalf("is_admin=TRUE user should get admin role: has=%v err=%v", hasAdmin, err)
	}
	normalHasAdmin, err := db.Permissions.UserHasRole(ctx, normalUser.ID, core.RoleAdmin)
	if err != nil || normalHasAdmin {
		t.Fatalf("is_admin=FALSE user should not get admin role: has=%v err=%v", normalHasAdmin, err)
	}
}

func TestSeedUserSubjectsMigratesIsSuperAdminColumnAndDedupExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	superUser := testutil.CreateUser(t, db, "migrate-super-first@test.com", "pw", "MigrateSuperFirst", false)
	secondSuper := testutil.CreateUser(t, db, "migrate-super-second@test.com", "pw", "MigrateSuperSecond", false)

	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT FALSE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET is_super_admin=TRUE WHERE id IN ($1,$2)`, superUser.ID, secondSuper.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("multiple is_super_admin=TRUE should be deduped to exactly one: got=%d", count)
	}
}

func TestSeedDefaultsFirstRegisteredUserBecomesSuperAdmin(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	user := testutil.CreateUser(t, db, "first-user@test.com", "pw", "FirstUser", false)
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("SeedDefaults should ensure exactly one super_admin after removal: got=%d", count)
	}
	hasSuper, err := db.Permissions.UserHasRole(ctx, user.ID, core.RoleSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSuper {
		t.Fatal("the only user should become super_admin when none exists")
	}
}

func TestSeedDefaultsDeduplicatesMultipleSuperAdminsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	first := testutil.CreateUser(t, db, "dedup-first@test.com", "pw", "DedupFirst", false)
	second := testutil.CreateUser(t, db, "dedup-second@test.com", "pw", "DedupSecond", false)
	third := testutil.CreateUser(t, db, "dedup-third@test.com", "pw", "DedupThird", false)
	now := time.Now().UnixMilli()

	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, sub string }{
		{first.ID, permissiondb.SubjectIDForUser(first.ID)},
		{second.ID, permissiondb.SubjectIDForUser(second.ID)},
		{third.ID, permissiondb.SubjectIDForUser(third.ID)},
	} {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO subject_roles (subject_id,role_id,created_at)
			VALUES ($1,$2,$3)
		`, u.sub, core.RoleSuperAdmin, now); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dedup should keep exactly one super_admin: got=%d", count)
	}
	var keptSubject string
	if err := db.Pool.QueryRow(ctx, `
		SELECT sr.subject_id FROM subject_roles sr
		JOIN permission_subjects ps ON ps.id=sr.subject_id
		JOIN users u ON u.id=ps.user_id
		WHERE sr.role_id=$1
		ORDER BY u.created_at ASC, u.id ASC
		LIMIT 1
	`, core.RoleSuperAdmin).Scan(&keptSubject); err != nil {
		t.Fatal(err)
	}
	if keptSubject != permissiondb.SubjectIDForUser(first.ID) {
		t.Fatalf("dedup should keep earliest user: got=%s want=%s", keptSubject, permissiondb.SubjectIDForUser(first.ID))
	}
}

func TestSessionPolicyReturnsErrorOnMissingTable(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "session-policy-err@test.com", "pw", "SessionPolicyErr", false)

	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindWeb,
		Entrypoint:  core.EntrypointDashboard,
	})
	if err == nil {
		t.Fatal("EffectivePermissionsForUser should fail when session_permission_policies is missing")
	}
}

func TestDelegationPolicyReturnsErrorOnMissingTable(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "delegation-policy-err@test.com", "pw", "DelegationPolicyErr", false)

	if _, err := db.Pool.Exec(ctx, `DROP TABLE delegated_permission_grants CASCADE`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:       core.SessionKindWeb,
		Entrypoint:        core.EntrypointDashboard,
		DelegatedGrantID:  "test-grant",
		DelegatedClientID: "test-client",
	})
	if err == nil {
		t.Fatal("EffectivePermissionsForUser should fail when delegated_permission_grants is missing")
	}
}

func TestEffectivePermissionsForUserCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	user := testutil.CreateUser(t, db, "cancelled-ctx@test.com", "pw", "CancelledCtx", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err == nil {
		t.Fatal("EffectivePermissionsForUser should fail with cancelled context")
	}
}

func TestActorForUserErrorFromPermissions(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.ActorForUser(ctx, "nonexistent", permissiondb.EffectiveOptions{})
	if err == nil {
		t.Fatal("ActorForUser should fail with cancelled context")
	}
}

func TestGrantRoleErrorFromEnsureUserSubject(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Permissions.GrantRole(ctx, "nonexistent", core.RoleModerator, "")
	if err == nil {
		t.Fatal("GrantRole should fail with cancelled context")
	}
}

func TestRevokeRoleErrorPath(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.RevokeRole(ctx, "nonexistent", core.RoleModerator)
	if err == nil {
		t.Fatal("RevokeRole should fail with cancelled context")
	}
}

func TestRoleIDsForUserCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.RoleIDsForUser(ctx, "nonexistent")
	if err == nil {
		t.Fatal("RoleIDsForUser should fail with cancelled context")
	}
}

func TestSetSubjectPermissionOverrideCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	def := core.MustDefinitionByCode("notice.create.any")
	err := db.Permissions.SetSubjectPermissionOverride(ctx, "nonexistent", def, "allow", "")
	if err == nil {
		t.Fatal("SetSubjectPermissionOverride should fail with cancelled context")
	}
}

func TestGrantInitialSuperAdminIfNoneErrorPath(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.GrantInitialSuperAdminIfNone(ctx, "nonexistent")
	if err == nil {
		t.Fatal("GrantInitialSuperAdminIfNone should fail with cancelled context")
	}
}

func TestSeedDefaultsFailsWhenCatalogTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_resources CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when permission_resources is missing")
	}
}

func TestSeedDefaultsFailsWhenRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when roles is missing")
	}
}

func TestSeedDefaultsFailsWhenPermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when permissions is missing")
	}
}

func TestSeedDefaultsFailsWhenSessionPoliciesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when session_permission_policies is missing")
	}
}

func TestSeedDefaultsFailsWhenSubjectRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE subject_roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when subject_roles is missing")
	}
}

func TestSeedDefaultsFailsWhenPermissionActionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_actions CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when permission_actions is missing")
	}
}

func TestSeedDefaultsFailsWhenRolePermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE role_permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when role_permissions is missing")
	}
}

func TestSeedDefaultsFailsWhenPermissionSubjectsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_subjects CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err == nil {
		t.Fatal("SeedDefaults should fail when permission_subjects is missing")
	}
}

func TestEnsureUserSubjectCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := db.Permissions.EnsureUserSubject(ctx, "nonexistent"); err == nil {
		t.Fatal("EnsureUserSubject should fail with cancelled context")
	}
}

func has(bits core.BitSet, code string) bool {
	return bits.Has(core.MustDefinitionByCode(code).BitIndex)
}
