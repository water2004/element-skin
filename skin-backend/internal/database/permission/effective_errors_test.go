package permission_test

import (
	"context"
	"errors"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

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
	if _, err := db.Pool.Exec(ctx, `DROP TRIGGER users_webhook_event ON users`); err != nil {
		t.Fatal(err)
	}
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

func TestEffectivePermissionsForClientAndCacheDependencyErrorsExactly(t *testing.T) {
	t.Run("client subject ensure error", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		db.Close()
		_, err := db.Permissions.EffectivePermissionsForClient(ctx, "closed-client", permissiondb.EffectiveOptions{})
		assertPgErrorOrClosed(t, err)
	})

	t.Run("actor for client propagates permission error", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		db.Close()
		_, err := db.Permissions.ActorForClient(ctx, "closed-actor-client", permissiondb.EffectiveOptions{})
		assertPgErrorOrClosed(t, err)
	})

	t.Run("cache read error", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "cache-read-error@test.com", "pw", "CacheReadError", false)
		cacheErr := errors.New("permission cache unavailable")
		cacheStore := redisstore.NewMemoryStore()
		cacheStore.Err = cacheErr
		db.Permissions.Cache = &permissiondb.RedisPermCache{Store: cacheStore}

		_, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if !errors.Is(err, cacheErr) {
			t.Fatalf("cache read error mismatch: got=%v want=%v", err, cacheErr)
		}
	})
}

func TestDelegationPolicyScanAndRowsErrorsExactly(t *testing.T) {
	for _, tc := range []struct {
		name string
		conn func(*testutil.FaultyConn) *testutil.FaultyConn
	}{
		{name: "scan", conn: func(fc *testutil.FaultyConn) *testutil.FaultyConn {
			return fc.WithScanErrorOnQuery(3, testutil.ErrFaultInjected)
		}},
		{name: "rows", conn: func(fc *testutil.FaultyConn) *testutil.FaultyConn {
			return fc.WithRowsErrOnQuery(3, testutil.ErrFaultInjected)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestAppTB(t)
			ctx := context.Background()
			owner := testutil.CreateUser(t, db, "delegation-"+tc.name+"@test.com", "pw", "Delegation"+tc.name, false)
			subjectClientID := "subject-client-" + tc.name
			resourceClientID := "resource-client-" + tc.name
			grantID := "grant-" + tc.name
			if err := db.Permissions.EnsureClientSubject(ctx, subjectClientID); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UnixMilli()
			def := core.MustDefinitionByCode("minecraft_session.hasjoined.server")
			if _, err := db.Pool.Exec(ctx, `
				INSERT INTO delegated_clients (id, owner_user_id, name, status, created_at, updated_at)
				VALUES ($1, $2, $3, 'active', $4, $4)
			`, resourceClientID, owner.ID, "Resource "+tc.name, now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Pool.Exec(ctx, `
				INSERT INTO delegated_client_permissions (client_id, permission_id, created_at)
				VALUES ($1, $2, $3)
			`, resourceClientID, int64(def.ID), now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Pool.Exec(ctx, `
				INSERT INTO delegated_permission_grants (id, user_id, subject_id, client_id, status, created_at)
				VALUES ($1, $2, $3, $4, 'active', $5)
			`, grantID, owner.ID, permissiondb.SubjectIDForClient(subjectClientID), resourceClientID, now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Pool.Exec(ctx, `
				INSERT INTO delegated_grant_permissions (grant_id, permission_id, created_at)
				VALUES ($1, $2, $3)
			`, grantID, int64(def.ID), now); err != nil {
				t.Fatal(err)
			}
			db.Permissions.SetTestConn(tc.conn(testutil.NewFaultyConn(db.Pool)))

			_, err := db.Permissions.EffectivePermissionsForClient(ctx, subjectClientID, permissiondb.EffectiveOptions{
				DelegatedClientID: resourceClientID,
				DelegatedGrantID:  grantID,
			})
			if !errors.Is(err, testutil.ErrFaultInjected) {
				t.Fatalf("delegation %s error mismatch: got=%v want=%v", tc.name, err, testutil.ErrFaultInjected)
			}
		})
	}
}
