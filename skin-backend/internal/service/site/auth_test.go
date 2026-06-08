package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRegisterCreatesFirstAdminAndOfflineProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := site.Site{DB: db, Cfg: testutil.TestConfig()}
	if err := db.SetSetting(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	userID, err := svc.Register(ctx, " auth-service@test.com ", "Password123", "AuthService", "", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.GetUserByID(ctx, userID)
	if err != nil || user == nil || !user.IsAdmin || user.Email != "auth-service@test.com" || user.DisplayName != "AuthService" {
		t.Fatalf("registered user mismatch: user=%#v err=%v", user, err)
	}
	profiles, err := db.GetProfilesByUser(ctx, userID, 10)
	if err != nil || len(profiles) != 1 || profiles[0].ID != util.OfflineUUIDNoDash("auth_service") || profiles[0].Name != "auth_service" {
		t.Fatalf("registration profile mismatch: profiles=%#v err=%v", profiles, err)
	}
}
