package token_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/token"
	"element-skin/backend/internal/testutil"
)

func TestStoreRefreshLifecycle(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := token.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-token@test.com", "Password123", "DomainToken", false)
	if err := store.AddRefresh(ctx, "domain_refresh", user.ID, 1000, 40); err != nil {
		t.Fatal(err)
	}
	refresh, err := store.GetRefresh(ctx, "domain_refresh")
	if err != nil || refresh["user_id"] != user.ID {
		t.Fatalf("refresh mismatch: refresh=%#v err=%v", refresh, err)
	}
	consumed, err := store.ConsumeRefresh(ctx, "domain_refresh")
	if err != nil || consumed["token_hash"] != "domain_refresh" {
		t.Fatalf("consume mismatch: refresh=%#v err=%v", consumed, err)
	}
	if again, err := store.ConsumeRefresh(ctx, "domain_refresh"); err != nil || again != nil {
		t.Fatalf("refresh should be single-use: refresh=%#v err=%v", again, err)
	}
}

func TestStoreRefreshDeletionPaths(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := token.Store{Pool: db.Pool}
	userA := testutil.CreateUser(t, db, "domain-refresh-delete-a@test.com", "Password123", "DomainRefreshDeleteA", false)
	userB := testutil.CreateUser(t, db, "domain-refresh-delete-b@test.com", "Password123", "DomainRefreshDeleteB", false)

	if err := store.AddRefresh(ctx, "refresh_one", userA.ID, 100, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddRefresh(ctx, "refresh_expired", userA.ID, 10, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.AddRefresh(ctx, "refresh_other_user", userB.ID, 100, 3); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRefresh(ctx, "refresh_one"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetRefresh(ctx, "refresh_one"); err != nil || got != nil {
		t.Fatalf("DeleteRefresh should remove exact token: refresh=%#v err=%v", got, err)
	}
	if err := store.DeleteExpiredRefresh(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetRefresh(ctx, "refresh_expired"); err != nil || got != nil {
		t.Fatalf("DeleteExpiredRefresh should remove expired token: refresh=%#v err=%v", got, err)
	}
	if got, err := store.GetRefresh(ctx, "refresh_other_user"); err != nil || got == nil || got["user_id"] != userB.ID {
		t.Fatalf("DeleteExpiredRefresh should keep unexpired other token: refresh=%#v err=%v", got, err)
	}
	if err := store.DeleteRefreshByUser(ctx, userB.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetRefresh(ctx, "refresh_other_user"); err != nil || got != nil {
		t.Fatalf("DeleteRefreshByUser should remove user refresh tokens: refresh=%#v err=%v", got, err)
	}
}

func TestStoreRotateRefreshIsAtomicAndSingleUse(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := token.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-refresh-rotate@test.com", "Password123", "DomainRefreshRotate", false)
	if err := store.AddRefresh(ctx, "rotate_old", user.ID, 1000, 10); err != nil {
		t.Fatal(err)
	}

	rotated, err := store.RotateRefresh(ctx, "rotate_old", "rotate_new", user.ID, 2000, 20)
	if err != nil || !rotated {
		t.Fatalf("rotate refresh = %v, %v; want true, nil", rotated, err)
	}
	if old, err := store.GetRefresh(ctx, "rotate_old"); err != nil || old != nil {
		t.Fatalf("successful rotation must remove old token: old=%#v err=%v", old, err)
	}
	newToken, err := store.GetRefresh(ctx, "rotate_new")
	if err != nil || newToken == nil ||
		newToken["user_id"] != user.ID ||
		newToken["expires_at"] != int64(2000) ||
		newToken["created_at"] != int64(20) {
		t.Fatalf("successful rotation stored unexpected token: token=%#v err=%v", newToken, err)
	}
	rotated, err = store.RotateRefresh(ctx, "rotate_old", "rotate_second", user.ID, 3000, 30)
	if err != nil || rotated {
		t.Fatalf("consumed token rotation = %v, %v; want false, nil", rotated, err)
	}
	if second, err := store.GetRefresh(ctx, "rotate_second"); err != nil || second != nil {
		t.Fatalf("single-use rotation must not insert another token: token=%#v err=%v", second, err)
	}
}

func TestStoreRotateRefreshRollsBackOldTokenWhenInsertFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := token.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-refresh-rollback@test.com", "Password123", "DomainRefreshRollback", false)
	if err := store.AddRefresh(ctx, "rollback_old", user.ID, 1000, 10); err != nil {
		t.Fatal(err)
	}
	if err := store.AddRefresh(ctx, "rollback_collision", user.ID, 1500, 15); err != nil {
		t.Fatal(err)
	}

	rotated, err := store.RotateRefresh(ctx, "rollback_old", "rollback_collision", user.ID, 2000, 20)
	if err == nil || rotated {
		t.Fatalf("colliding rotation = %v, %v; want false and insert error", rotated, err)
	}
	old, err := store.GetRefresh(ctx, "rollback_old")
	if err != nil || old == nil ||
		old["user_id"] != user.ID ||
		old["expires_at"] != int64(1000) ||
		old["created_at"] != int64(10) {
		t.Fatalf("failed rotation must restore exact old token: token=%#v err=%v", old, err)
	}
	collision, err := store.GetRefresh(ctx, "rollback_collision")
	if err != nil || collision == nil ||
		collision["expires_at"] != int64(1500) ||
		collision["created_at"] != int64(15) {
		t.Fatalf("failed rotation must preserve existing colliding token: token=%#v err=%v", collision, err)
	}
}
