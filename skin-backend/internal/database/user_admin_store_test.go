package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestUserAdminStoreToggleBanAndUnbanExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "user-admin-store@test.com", "Password123", "UserAdminStore", false)
	next, err := db.ToggleAdmin(ctx, user.ID)
	if err != nil || !next {
		t.Fatalf("ToggleAdmin should enable admin: next=%v err=%v", next, err)
	}
	until := database.NowMS() + 60_000
	if err := db.BanUser(ctx, user.ID, until); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.IsBanned(ctx, user.ID); err != nil || !banned {
		t.Fatalf("user should be banned: banned=%v err=%v", banned, err)
	}
	if err := db.UnbanUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.IsBanned(ctx, user.ID); err != nil || banned {
		t.Fatalf("user should be unbanned: banned=%v err=%v", banned, err)
	}
}
