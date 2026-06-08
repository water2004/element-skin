package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestDBInitResetAndCoreHelpersExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Init should be idempotent: %v", err)
	}
	if siteName, err := db.GetSetting(ctx, "site_name", ""); err != nil || siteName != "皮肤站" {
		t.Fatalf("Init should seed site_name: site_name=%q err=%v", siteName, err)
	}
	testutil.CreateUser(t, db, "db-reset@test.com", "Password123", "DBReset", false)
	if err := db.ResetPublicSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := db.CountUsers(ctx); err != nil || count != 0 {
		t.Fatalf("reset should remove users: count=%d err=%v", count, err)
	}
	if !database.IsNoRows(pgx.ErrNoRows) || database.IsNoRows(nil) {
		t.Fatal("IsNoRows should match pgx.ErrNoRows only")
	}
	if now := database.NowMS(); now <= 0 {
		t.Fatalf("NowMS should be positive: %d", now)
	}
}
