package auth_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestAuthRegisterConsumesVerificationAndInviteExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "INVITE_ONCE", testutil.Pointer(1), "Invite Once"); err != nil {
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
	if stored, err := svc.Redis.GetVerificationCode(ctx, "second-register@test.com", "register"); err != nil || stored != "SECOND12" {
		t.Fatalf("failed registration must preserve the verification code for retry: code=%q err=%v", stored, err)
	}
}

func TestAuthRegisterRejectsMissingVerificationAndInviteInputsWithoutSideEffects(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		email      string
		username   string
		invite     string
		code       string
		storedCode string
		want       string
	}{
		{
			name:     "verification code required",
			email:    "register-code-required@test.com",
			username: "RegisterCodeRequired",
			invite:   "UNKNOWN",
			want:     "Verification code required",
		},
		{
			name:       "verification code invalid",
			email:      "register-code-invalid@test.com",
			username:   "RegisterCodeInvalid",
			invite:     "UNKNOWN",
			code:       "WRONG",
			storedCode: "RIGHT123",
			want:       "Invalid or expired verification code",
		},
		{
			name:       "invite code required",
			email:      "register-invite-required@test.com",
			username:   "RegisterInviteRequired",
			code:       "VALID123",
			storedCode: "VALID123",
			want:       "invite code required",
		},
		{
			name:       "invite code invalid",
			email:      "register-invite-invalid@test.com",
			username:   "RegisterInviteInvalid",
			invite:     "UNKNOWN",
			code:       "VALID456",
			storedCode: "VALID456",
			want:       "invalid invite code",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.storedCode != "" {
				if err := svc.Redis.SetVerificationCode(ctx, tc.email, "register", tc.storedCode, 0); err != nil {
					t.Fatal(err)
				}
			}
			id, err := svc.Register(ctx, tc.email, "Password123", tc.username, tc.invite, tc.code)
			if id != "" || !httpError(err, 400, tc.want) {
				t.Fatalf("Register() id=%q err=%#v; want empty id and exact %q", id, err, tc.want)
			}
			if user, err := db.Users.GetByEmail(ctx, tc.email); err != nil || user != nil {
				t.Fatalf("rejected registration created user=%#v err=%v", user, err)
			}
			if tc.storedCode != "" {
				stored, err := svc.Redis.GetVerificationCode(ctx, tc.email, "register")
				if err != nil || stored != tc.storedCode {
					t.Fatalf("rejected registration changed code=%q err=%v; want %q", stored, err, tc.storedCode)
				}
			}
		})
	}
	if count, err := db.Users.Count(ctx); err != nil || count != 0 {
		t.Fatalf("rejected registrations left user count=%d err=%v; want 0", count, err)
	}
}
