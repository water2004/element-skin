package database_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUserStoreCreateUpdatePasswordDeleteAndInviteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.CreateInvite(ctx, "user_store_invite", 1, "User Store Invite"); err != nil {
		t.Fatal(err)
	}
	hash, err := util.HashPassword("Password123")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: "user_store_direct", Email: "user-store-direct@test.com", Password: hash, DisplayName: "UserStoreDirect"}
	profile := model.Profile{ID: "user_store_direct_profile", UserID: user.ID, Name: "UserStoreDirectProfile", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, user, profile, "user_store_invite", user.Email); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUserWithProfile(ctx, model.User{ID: "user_store_direct_2", Email: "user-store-direct-2@test.com", Password: hash}, model.Profile{ID: "user_store_direct_profile_2", UserID: "user_store_direct_2", Name: "UserStoreDirectProfile2", TextureModel: "default"}, "user_store_invite", "second"); !errors.Is(err, database.ErrInviteExhausted) {
		t.Fatalf("exhausted invite should fail with ErrInviteExhausted, got %v", err)
	}
	if err := db.UpdateUser(ctx, user.ID, map[string]any{"email": "updated-user-store@test.com", "display_name": "UpdatedUserStore"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetUserByEmail(ctx, "updated-user-store@test.com")
	if err != nil || got == nil || got.ID != user.ID || got.DisplayName != "UpdatedUserStore" {
		t.Fatalf("updated user mismatch: user=%#v err=%v", got, err)
	}
	newHash, err := util.HashPassword("NewPassword123")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := db.UpdatePasswordAndRevokeRefresh(ctx, user.ID, newHash)
	if err != nil || !updated {
		t.Fatalf("password update mismatch: updated=%v err=%v", updated, err)
	}
	deleted, err := db.DeleteUser(ctx, user.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteUser mismatch: deleted=%v err=%v", deleted, err)
	}
	if gone, err := db.GetUserByID(ctx, user.ID); err != nil || gone != nil {
		t.Fatalf("deleted user should be gone: user=%#v err=%v", gone, err)
	}
}
