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
