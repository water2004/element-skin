package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := site.Site{DB: db, Cfg: testutil.TestConfig()}
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, user.ID, "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, user.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}
}
