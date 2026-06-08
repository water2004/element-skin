package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestAccountMeReturnsCountsAndUpdateMePersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := site.Site{DB: db, Cfg: testutil.TestConfig()}
	user := testutil.CreateUser(t, db, "site-account-service@test.com", "Password123", "SiteAccountService", false)

	if err := svc.UpdateMe(ctx, user.ID, map[string]any{"email": "updated-account@test.com", "display_name": "UpdatedAccount", "preferred_language": "en_US", "avatar_hash": "avatar_hash"}); err != nil {
		t.Fatal(err)
	}
	me, err := svc.Me(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if me["email"] != "updated-account@test.com" || me["display_name"] != "UpdatedAccount" || me["lang"] != "en_US" ||
		me["profile_count"] != 0 || me["texture_count"] != 0 {
		t.Fatalf("Me response mismatch: %#v", me)
	}
}
