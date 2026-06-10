package site_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestVerificationSendAndVerifyExactStoredCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", "180"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SendVerificationCode(ctx, "verify-service@test.com", "register")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true || res["ttl"] != 180 {
		t.Fatalf("verification response mismatch: %#v", res)
	}
	code, err := svc.Redis.GetVerificationCode(ctx, "verify-service@test.com", "register")
	if err != nil || len(code) != 8 || strings.ToUpper(code) != code {
		t.Fatalf("stored verification code mismatch: code=%q err=%v", code, err)
	}
	verified, err := svc.VerifyCode(ctx, "verify-service@test.com", strings.ToLower(code), "register")
	if err != nil || !verified {
		t.Fatalf("VerifyCode should be case-insensitive: verified=%v err=%v", verified, err)
	}
}

func TestVerificationRejectsInvalidRequestsAndHidesMissingResetAccount(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "verify-existing@test.com", "Password123", "VerifyExisting", false)

	if _, err := svc.SendVerificationCode(ctx, "verify-new@test.com", "register"); !httpError(err, 400, "Email verification is disabled") {
		t.Fatalf("disabled verification should reject exactly, got %#v", err)
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SendVerificationCode(ctx, "missing-reset@test.com", "reset")
	if err != nil || res["ok"] != true || res["ttl"] != 0 {
		t.Fatalf("missing reset account should return generic ok without code: res=%#v err=%v", res, err)
	}
	if _, err := svc.Redis.GetVerificationCode(ctx, "missing-reset@test.com", "reset"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("missing reset account must not store verification code, got %v", err)
	}

	for _, tc := range []struct {
		name  string
		email string
		typ   string
		want  string
	}{
		{"bad email", "not-an-email", "register", "Invalid email format"},
		{"registered email", "verify-existing@test.com", "register", "Email already registered"},
		{"bad type", "verify-new@test.com", "bad", "invalid verification type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.SendVerificationCode(ctx, tc.email, tc.typ); !httpError(err, 400, tc.want) {
				t.Fatalf("SendVerificationCode should reject %s exactly, got %#v", tc.name, err)
			}
		})
	}

	ok, err := svc.VerifyCode(ctx, "verify-new@test.com", "missing", "register")
	if err != nil || ok {
		t.Fatalf("missing verification code should return false without error: ok=%v err=%v", ok, err)
	}
}

func TestResetPasswordRejectsDisabledWeakAndBadCodesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "verify-reset@test.com", "Password123", "VerifyReset", false)

	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "NO_CODE"); !httpError(err, 403, "Password reset via email is disabled") {
		t.Fatalf("disabled reset should reject exactly, got %#v", err)
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Redis.SetVerificationCode(ctx, user.Email, "reset", "RESET123", 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(ctx, user.Email, "weak", "RESET123"); err == nil {
		t.Fatal("strong password policy should reject weak reset password")
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("weak reset password must not change hash: user=%#v err=%v", unchanged, err)
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", "false"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "WRONG"); !httpError(err, 400, "Invalid or expired verification code") {
		t.Fatalf("bad reset code should reject exactly, got %#v", err)
	}
}

func TestResetPasswordMissingAccountPreservesVerificationCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	const email = "missing-reset-account@test.com"
	const code = "RESET404"
	if err := svc.Redis.SetVerificationCode(ctx, email, "reset", code, 0); err != nil {
		t.Fatal(err)
	}

	err := svc.ResetPassword(ctx, email, "NewPassword123", code)
	if !httpError(err, 404, "User not found") {
		t.Fatalf("missing reset account should reject exactly, got %#v", err)
	}
	stored, err := svc.Redis.GetVerificationCode(ctx, email, "reset")
	if err != nil || stored != code {
		t.Fatalf("failed reset must preserve code for its remaining TTL: code=%q err=%v", stored, err)
	}
}
