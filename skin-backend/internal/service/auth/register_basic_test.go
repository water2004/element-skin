package auth_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRegisterCreatesFirstAdminAndOfflineProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	userID, err := svc.Register(ctx, " auth-service@test.com ", "Password123", "AuthService", "", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil || user == nil || user.Email != "auth-service@test.com" || user.DisplayName != "AuthService" {
		t.Fatalf("registered user mismatch: user=%#v err=%v", user, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, userID); err != nil || !protected {
		t.Fatalf("first registered user should become protected subject: protected=%v err=%v", protected, err)
	}
	profiles, err := db.Profiles.GetByUser(ctx, userID, 10)
	if err != nil || len(profiles) != 1 || profiles[0].ID != util.OfflineUUIDNoDash("auth_service") || profiles[0].Name != "auth_service" {
		t.Fatalf("registration profile mismatch: profiles=%#v err=%v", profiles, err)
	}
}

func TestAuthRegisterRejectsPolicyFailuresWithoutCreatingUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())

	if err := db.Settings.Set(ctx, "allow_register", "false"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "closed-register@test.com", "Password123", "ClosedRegister", "", ""); !httpError(err, 403, "registration.create.disabled") {
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
	if _, err := svc.Register(ctx, "weak-register@test.com", "weak", "WeakRegister", "", ""); !httpError(err, 400, "password.validate.invalid") {
		t.Fatalf("strong password policy should reject weak password with exact HTTP 400, got %#v", err)
	} else {
		var httpErr util.HTTPError
		if !errors.As(err, &httpErr) || !reflect.DeepEqual(httpErr.Params, map[string]any{"rules": []string{"min_length", "uppercase", "number"}}) {
			t.Fatalf("strong password rules mismatch: %#v", err)
		}
	}
	if user, err := db.Users.GetByEmail(ctx, "weak-register@test.com"); err != nil || user != nil {
		t.Fatalf("weak password registration must not create user: user=%#v err=%v", user, err)
	}
}

func TestAuthRejectsInvalidCredentialsAndRegistrationIdentityConflicts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
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
			if !httpError(err, 401, "credentials.verify.invalid") || res != nil {
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
		{name: "missing username", email: "missing-name@test.com", username: "   ", want: "username.validate.required"},
		{name: "invalid email", email: "not-an-email", username: "ValidName", want: "email.validate.invalid"},
		{name: "duplicate username", email: "new-email@test.com", username: existing.DisplayName, want: "username.reserve.conflict"},
		{name: "duplicate email", email: existing.Email, username: "DifferentName", want: "email.register.already_exists"},
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

func TestRegisterStopsAfterGeneratedProfileNameCandidatesAreExhausted(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "profile-name-exhaust-owner@test.com", "Password123", "ProfileNameExhaustOwner", false)
	for attempt := 0; attempt < 100; attempt++ {
		testutil.CreateProfile(
			t,
			db,
			owner.ID,
			"profile_name_exhaust_"+strconv.Itoa(attempt),
			util.ProfileNameCandidate("collision", attempt),
		)
	}

	userID, err := svc.Register(ctx, "collision@new.test", "Password123", "ProfileNameExhaustNew", "", "")
	if userID != "" || !httpError(err, 500, "profile_name.allocate.failed") {
		t.Fatalf("exhausted generated names: user_id=%q err=%#v, want empty id and exact 500", userID, err)
	}
	if user, err := db.Users.GetByEmail(ctx, "collision@new.test"); err != nil || user != nil {
		t.Fatalf("exhausted registration must not create user: user=%#v err=%v", user, err)
	}
	if count, err := db.Profiles.CountByUser(ctx, owner.ID); err != nil || count != 100 {
		t.Fatalf("exhausted registration changed existing profiles: count=%d err=%v", count, err)
	}
}
