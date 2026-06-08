package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestSessionRotateRefreshIsSingleUse(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := site.Site{DB: db, Cfg: cfg}
	testutil.CreateUser(t, db, "site-session-service@test.com", "Password123", "SiteSessionService", false)
	login, err := svc.Login(ctx, "site-session-service@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["refresh_token"] == "" || rotated["refresh_token"] == login["refresh_token"] {
		t.Fatalf("rotated refresh should be new and non-empty: %#v", rotated)
	}
	if _, err := svc.RotateRefresh(ctx, login["refresh_token"].(string)); err == nil {
		t.Fatal("old refresh token should be consumed")
	}
}
