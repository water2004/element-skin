package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestProfileListStorePaginatesUserAndAdminListsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	first := testutil.CreateUser(t, db, "profile-list-first@test.com", "Password123", "ProfileListFirst", false)
	second := testutil.CreateUser(t, db, "profile-list-second@test.com", "Password123", "ProfileListSecond", false)
	testutil.CreateProfile(t, db, first.ID, "profile_list_a", "ProfileListAlpha")
	testutil.CreateProfile(t, db, second.ID, "profile_list_b", "ProfileListBeta")

	userPage, err := db.ListProfilesByUser(ctx, first.ID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	userItems := userPage["items"].([]map[string]any)
	if len(userItems) != 1 || userItems[0]["id"] != "profile_list_a" || userPage["has_next"] != false || userPage["next_key"] != nil {
		t.Fatalf("user profile page mismatch: %#v", userPage)
	}

	adminPage, err := db.ListAllProfiles(ctx, 1, "", "ProfileList")
	if err != nil {
		t.Fatal(err)
	}
	adminItems := adminPage["items"].([]map[string]any)
	if len(adminItems) != 1 || adminItems[0]["id"] != "profile_list_a" || adminItems[0]["owner_email"] != "profile-list-first@test.com" || adminPage["has_next"] != true {
		t.Fatalf("admin profile page mismatch: %#v", adminPage)
	}
}
