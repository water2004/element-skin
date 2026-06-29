package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

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

func TestExpiredBanRestoresJoinPermission(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "expired-ban@test.com", "pw", "ExpiredBan", false)
	expiredUntil := time.Now().Add(-time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, expiredUntil, user.ID); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, "yggdrasil_server.join.bound_profile") {
		t.Fatal("expired ban should restore join permission")
	}
	if !has(bits, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("expired ban should keep hasjoined permission")
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

func TestEffectivePermissionsRejectsNonexistentUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	opts := permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	}
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, "nonexistent-ban-check", opts)
	assertPostgresError(t, err, "23503")
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

func TestSessionPolicyUsesCacheNotDatabase(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "session-policy-cache@test.com", "pw", "SessionPolicyCache", false)

	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindWeb,
		Entrypoint:  core.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("session policy should use in-memory cache, not DB: %v", err)
	}
	if !has(bits, "profile.create.owned") {
		t.Fatal("cached web session policy should include profile.create.owned")
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
	assertPostgresError(t, err, "42P01")
}

func TestEffectivePermissionsForUserCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	user := testutil.CreateUser(t, db, "cancelled-ctx@test.com", "pw", "CancelledCtx", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	assertCancelled(t, err)
}

func TestEffectivePermissionsReturnsErrorWhenOverridesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "effective-overrides-missing@test.com", "pw", "EffectiveOverridesMissing", false)
	if _, err := db.Pool.Exec(ctx, `DROP TABLE subject_permission_overrides CASCADE`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	assertPostgresError(t, err, "42P01")
}

func TestActorForUserErrorFromPermissions(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.ActorForUser(ctx, "nonexistent", permissiondb.EffectiveOptions{})
	assertCancelled(t, err)
}

func TestEffectivePermissionsWithBanPolicyColumnTypeError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "col-type-err@test.com", "pw", "ColTypeErr", false)
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users ALTER COLUMN banned_until TYPE TEXT USING COALESCE(banned_until::TEXT, '')`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		ApplyBanPolicy: true,
	})
	assertPgErrorOrClosed(t, err)
}

func TestPoolClosedReturnsError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "pool-closed@test.com", "pw", "PoolClosed", false)
	db.Pool.Close()
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	assertPgErrorOrClosed(t, err)
}

func TestEffectivePermissionsRowsCanError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "rows-scan-err@test.com", "pw", "RowsScanErr", false)
	fc := testutil.NewFaultyConn(db.Pool).WithScanError(testutil.ErrFaultInjected)
	db.Permissions.SetTestConn(fc)
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != testutil.ErrFaultInjected {
		t.Fatalf("should return injected Scan error: %v", err)
	}
}

func TestEffectivePermissionsRowsErr(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "rows-err-err@test.com", "pw", "RowsErrErr", false)
	fc := testutil.NewFaultyConn(db.Pool).WithRowsErr(testutil.ErrFaultInjected)
	db.Permissions.SetTestConn(fc)
	_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != testutil.ErrFaultInjected {
		t.Fatalf("should return injected Err error: %v", err)
	}
}
