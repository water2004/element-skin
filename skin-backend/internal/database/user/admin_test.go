package user_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestAdminTogglesBanAndUnban(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	u := testutil.CreateUser(t, db, "domain-user-admin@test.com", "Password123", "DomainUserAdmin", false)
	next, err := store.ToggleAdmin(ctx, u.ID)
	if err != nil || !next {
		t.Fatalf("toggle should enable admin: next=%v err=%v", next, err)
	}
	if err := store.Ban(ctx, u.ID, 9_999_999_999_999); err != nil {
		t.Fatal(err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || !banned {
		t.Fatalf("user should be banned: banned=%v err=%v", banned, err)
	}
	if err := store.Unban(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || banned {
		t.Fatalf("user should be unbanned: banned=%v err=%v", banned, err)
	}
	if err := store.Ban(ctx, "missing-user", 9_999_999_999_999); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user ban error = %v; want pgx.ErrNoRows", err)
	}
	if err := store.Unban(ctx, "missing-user"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user unban error = %v; want pgx.ErrNoRows", err)
	}
}

func TestAdminTransferSuperAdminAndProtectSuperAdminToggle(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	superAdmin := testutil.CreateUser(t, db, "domain-user-super@test.com", "Password123", "DomainUserSuper", true, true)
	target := testutil.CreateUser(t, db, "domain-user-super-target@test.com", "Password123", "DomainUserSuperTarget", false)

	next, err := store.ToggleAdmin(ctx, superAdmin.ID)
	if err != nil || !next {
		t.Fatalf("toggling super admin should keep admin status: next=%v err=%v", next, err)
	}
	stillSuper, err := store.GetByID(ctx, superAdmin.ID)
	if err != nil || stillSuper == nil || !stillSuper.IsAdmin || !stillSuper.IsSuperAdmin {
		t.Fatalf("super admin toggle must not demote super admin: user=%#v err=%v", stillSuper, err)
	}

	if err := store.TransferSuperAdmin(ctx, superAdmin.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	oldSuper, err := store.GetByID(ctx, superAdmin.ID)
	if err != nil || oldSuper == nil || oldSuper.IsSuperAdmin || !oldSuper.IsAdmin {
		t.Fatalf("old super admin should remain admin only: user=%#v err=%v", oldSuper, err)
	}
	newSuper, err := store.GetByID(ctx, target.ID)
	if err != nil || newSuper == nil || !newSuper.IsSuperAdmin || !newSuper.IsAdmin {
		t.Fatalf("target should become admin and super admin: user=%#v err=%v", newSuper, err)
	}
	if err := store.TransferSuperAdmin(ctx, superAdmin.ID, "missing-user"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("transfer from non-super admin should fail with no rows, got %v", err)
	}
}

func TestTransferSuperAdminRollsBackWhenTargetDoesNotExist(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	superAdmin := testutil.CreateUser(t, db, "domain-transfer-rollback@test.com", "Password123", "TransferRollback", true, true)

	if err := store.TransferSuperAdmin(ctx, superAdmin.ID, "missing-target"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing target should return pgx.ErrNoRows, got %v", err)
	}
	unchanged, err := store.GetByID(ctx, superAdmin.ID)
	if err != nil || unchanged == nil || !unchanged.IsAdmin || !unchanged.IsSuperAdmin {
		t.Fatalf("failed transfer must roll back source demotion: user=%#v err=%v", unchanged, err)
	}
}

func TestConcurrentAdminTogglesApplyEveryToggle(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "domain-toggle-race@test.com", "Password123", "ToggleRace", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_admin_toggle() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_admin_toggle
		BEFORE UPDATE OF is_admin ON users
		FOR EACH ROW EXECUTE FUNCTION delay_admin_toggle();
	`); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			next, err := store.ToggleAdmin(context.Background(), target.ID)
			results <- next
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	trueResults := 0
	falseResults := 0
	for next := range results {
		if next {
			trueResults++
		} else {
			falseResults++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent toggle failed: %v", err)
		}
	}
	if trueResults != 1 || falseResults != 1 {
		t.Fatalf("concurrent toggle results: true=%d false=%d; want one of each", trueResults, falseResults)
	}
	updated, err := store.GetByID(ctx, target.ID)
	if err != nil || updated == nil || updated.IsAdmin {
		t.Fatalf("two toggles must restore non-admin state: user=%#v err=%v", updated, err)
	}
}
