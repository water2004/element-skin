package site_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRegisterCreatesFirstAdminAndOfflineProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	userID, err := svc.Register(ctx, " auth-service@test.com ", "Password123", "AuthService", "", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil || user == nil || !user.IsAdmin || !user.IsSuperAdmin || user.Email != "auth-service@test.com" || user.DisplayName != "AuthService" {
		t.Fatalf("registered user mismatch: user=%#v err=%v", user, err)
	}
	profiles, err := db.Profiles.GetByUser(ctx, userID, 10)
	if err != nil || len(profiles) != 1 || profiles[0].ID != util.OfflineUUIDNoDash("auth_service") || profiles[0].Name != "auth_service" {
		t.Fatalf("registration profile mismatch: profiles=%#v err=%v", profiles, err)
	}
}

func TestAuthRegisterRejectsPolicyFailuresWithoutCreatingUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())

	if err := db.Settings.Set(ctx, "allow_register", "false"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "closed-register@test.com", "Password123", "ClosedRegister", "", ""); !httpError(err, 403, "registration is disabled") {
		t.Fatalf("closed registration should reject exactly, got %#v", err)
	}
	if user, err := db.Users.GetByEmail(ctx, "closed-register@test.com"); err != nil || user != nil {
		t.Fatalf("closed registration must not create user: user=%#v err=%v", user, err)
	}

	if err := db.Settings.Set(ctx, "allow_register", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "weak-register@test.com", "weak", "WeakRegister", "", ""); err == nil {
		t.Fatal("strong password policy should reject weak registration password")
	}
	if user, err := db.Users.GetByEmail(ctx, "weak-register@test.com"); err != nil || user != nil {
		t.Fatalf("weak password registration must not create user: user=%#v err=%v", user, err)
	}
}

func TestAuthRegisterConsumesVerificationAndInviteExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "INVITE_ONCE", 1, "Invite Once"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Redis.SetVerificationCode(ctx, "verified-register@test.com", "register", "ABC12345", 0); err != nil {
		t.Fatal(err)
	}

	userID, err := svc.Register(ctx, "verified-register@test.com", "Password123", "VerifiedRegister", "INVITE_ONCE", "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if user, err := db.Users.GetByID(ctx, userID); err != nil || user == nil || user.Email != "verified-register@test.com" || user.DisplayName != "VerifiedRegister" {
		t.Fatalf("verified invite registration should create user: user=%#v err=%v", user, err)
	}
	if _, err := svc.Redis.GetVerificationCode(ctx, "verified-register@test.com", "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful register should consume verification code, got %v", err)
	}
	invite, err := db.Invites.Get(ctx, "INVITE_ONCE")
	if err != nil || invite == nil || invite.UsedCount != 1 || invite.UsedBy == nil || *invite.UsedBy != "verified-register@test.com" {
		t.Fatalf("successful register should consume invite exactly: invite=%#v err=%v", invite, err)
	}

	if err := svc.Redis.SetVerificationCode(ctx, "second-register@test.com", "register", "SECOND12", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "second-register@test.com", "Password123", "SecondRegister", "INVITE_ONCE", "SECOND12"); !httpError(err, 400, "invite code has no remaining uses") {
		t.Fatalf("exhausted invite should reject exactly, got %#v", err)
	}
	if user, err := db.Users.GetByEmail(ctx, "second-register@test.com"); err != nil || user != nil {
		t.Fatalf("exhausted invite must not create user: user=%#v err=%v", user, err)
	}
}

func TestAuthRejectsInvalidCredentialsAndRegistrationIdentityConflicts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	existing := testutil.CreateUser(t, db, "auth-existing@test.com", "Password123", "AuthExisting", false)

	for _, tc := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "missing account", email: "missing-auth@test.com", password: "Password123"},
		{name: "wrong password", email: existing.Email, password: "WrongPassword"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Login(ctx, tc.email, tc.password)
			if !httpError(err, 401, "Invalid credentials") || res != nil {
				t.Fatalf("Login(%s) should reject exactly: res=%#v err=%#v", tc.name, res, err)
			}
		})
	}

	for _, tc := range []struct {
		name     string
		email    string
		username string
		want     string
	}{
		{name: "missing username", email: "missing-name@test.com", username: "   ", want: "Username is required"},
		{name: "invalid email", email: "not-an-email", username: "ValidName", want: "Invalid email format"},
		{name: "duplicate username", email: "new-email@test.com", username: existing.DisplayName, want: "Username already exists"},
		{name: "duplicate email", email: existing.Email, username: "DifferentName", want: "Email already registered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := svc.Register(ctx, tc.email, "Password123", tc.username, "", "")
			if !httpError(err, 400, tc.want) || id != "" {
				t.Fatalf("Register(%s) should reject exactly: id=%q err=%#v", tc.name, id, err)
			}
		})
	}

	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("rejected auth attempts must not create users: count=%d err=%v", count, err)
	}
}
