package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestUserListStoreSearchesUsersAndProfilesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "user-list-store@test.com", "Password123", "UserListStore", false)
	testutil.CreateProfile(t, db, user.ID, "user_list_profile", "SearchByProfileName")
	list, err := db.ListUsers(ctx, 1, "", "SearchByProfileName")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != user.ID || items[0]["email"] != "user-list-store@test.com" || list["has_next"] != false || list["next_key"] != nil {
		t.Fatalf("user list search mismatch: %#v", list)
	}
}
