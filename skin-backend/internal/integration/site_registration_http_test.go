package integration_test

import (
	"context"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"strings"
	"testing"
)

func TestRegistrationRestrictionsAndInviteConsumption(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	first := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "admin-first@test.com", "password": "Password123", "username": "FirstAdmin"})
	if first.Code != http.StatusCreated {
		t.Fatalf("first register status=%d body=%s", first.Code, first.Body.String())
	}
	firstUser, err := db.Users.GetByEmail(ctx, "admin-first@test.com")
	if err != nil || firstUser == nil {
		t.Fatalf("first registered user should exist: user=%#v err=%v", firstUser, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, firstUser.ID); err != nil || !protected {
		t.Fatalf("first registered user should be protected: protected=%v err=%v", protected, err)
	}
	secondRegister := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "SecondNormal"})
	if secondRegister.Code != http.StatusCreated {
		t.Fatalf("second register status=%d body=%s", secondRegister.Code, secondRegister.Body.String())
	}
	secondUser, err := db.Users.GetByEmail(ctx, "second-normal@test.com")
	if err != nil || secondUser == nil {
		t.Fatalf("second registered user should exist: user=%#v err=%v", secondUser, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, secondUser.ID); err != nil || protected {
		t.Fatalf("second registered user should not be protected: protected=%v err=%v", protected, err)
	}
	duplicateEmail := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "second-normal@test.com", "password": "Password123", "username": "DuplicateEmailUser"})
	if duplicateEmail.Code != 400 || !strings.Contains(duplicateEmail.Body.String(), "Email already registered") {
		t.Fatalf("duplicate email should be rejected, got %d body=%s", duplicateEmail.Code, duplicateEmail.Body.String())
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", true); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	for _, weak := range []string{"12345", "simplepass"} {
		resp := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "weak_" + weak + "@test.com", "password": weak, "username": "Weak" + weak})
		if resp.Code != 400 {
			t.Fatalf("weak password %q should be rejected, got %d body=%s", weak, resp.Code, resp.Body.String())
		}
	}
	strong := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "strong@test.com", "password": "StrongP@ss1", "username": "StrongUser"})
	if strong.Code != http.StatusCreated {
		t.Fatalf("strong password should register, got %d body=%s", strong.Code, strong.Body.String())
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	for _, badEmail := range []string{"a@b", "a@x.com\r\nBcc: x@y.com", "notanemail"} {
		bad := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": badEmail, "password": "Password123!", "username": "SomeUser"})
		if bad.Code != 400 || !strings.Contains(bad.Body.String(), "Invalid email format") {
			t.Fatalf("invalid email %q should be rejected, got %d %s", badEmail, bad.Code, bad.Body.String())
		}
		if row, err := db.Users.GetByEmail(ctx, badEmail); err != nil || row != nil {
			t.Fatalf("invalid email registration should not create user: row=%#v err=%v", row, err)
		}
	}
	if err := db.Settings.Set(ctx, "allow_register", false); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	disabled := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if disabled.Code != 403 {
		t.Fatalf("disabled register should be 403, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.Settings.Set(ctx, "allow_register", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", true); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)
	missingInvite := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "x@test.com", "password": "Password123", "username": "XUser"})
	if missingInvite.Code != 400 {
		t.Fatalf("missing invite should be 400, got %d", missingInvite.Code)
	}
	if err := db.Invites.Create(ctx, "VALID_CODE", testutil.Pointer(1), "once"); err != nil {
		t.Fatal(err)
	}
	ok := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "first@test.com", "password": "Password123", "username": "FirstUser", "invite": "VALID_CODE"})
	if ok.Code != http.StatusCreated {
		t.Fatalf("valid invite register status=%d body=%s", ok.Code, ok.Body.String())
	}
	overuse := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "second@test.com", "password": "Password123", "username": "SecondUser", "invite": "VALID_CODE"})
	if overuse.Code != 400 {
		t.Fatalf("overused invite should be 400, got %d body=%s", overuse.Code, overuse.Body.String())
	}
	second, _ := db.Users.GetByEmail(ctx, "second@test.com")
	if second != nil {
		t.Fatal("overused invite should not create user")
	}
}

func TestVerificationCodeRegisterAndResetPasswordHTTP(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	disabled := doJSON(t, h, "POST", "/v2/auth/verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if disabled.Code != 400 {
		t.Fatalf("verification disabled should be 400, got %d body=%s", disabled.Code, disabled.Body.String())
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", 300); err != nil {
		t.Fatal(err)
	}
	invalidateSettings(t, redis)

	send := doJSON(t, h, "POST", "/v2/auth/verification-code", map[string]any{"email": "verify@test.com", "type": "register"})
	if send.Code != 200 {
		t.Fatalf("send verification status=%d body=%s", send.Code, send.Body.String())
	}
	sendBody := parseJSON(t, send)
	if len(sendBody) != 1 || sendBody["ttl"] != float64(300) {
		t.Fatalf("unexpected verification response: %#v", sendBody)
	}
	code, err := redis.GetVerificationCode(ctx, "verify@test.com", "register")
	if err != nil {
		t.Fatalf("verification code missing err=%v", err)
	}
	if len(code) != 8 {
		t.Fatalf("bad verification code code=%q", code)
	}
	if _, _, ok, err := db.Verifications.GetCode(ctx, "verify@test.com", "register"); err != nil || ok {
		t.Fatalf("verification code should not be persisted in db: ok=%v err=%v", ok, err)
	}
	for _, ch := range code {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
			t.Fatalf("verification code contains invalid character: %q", code)
		}
	}

	badRegister := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "verify@test.com", "password": "Password123!", "username": "VerifyUser", "code": "WRONG"})
	if badRegister.Code != 400 {
		t.Fatalf("wrong verification code should be 400, got %d body=%s", badRegister.Code, badRegister.Body.String())
	}
	register := doJSON(t, h, "POST", "/v2/auth/register", map[string]any{"email": "verify@test.com", "password": "Password123!", "username": "VerifyUser", "code": strings.ToLower(code)})
	if register.Code != http.StatusCreated {
		t.Fatalf("verified register status=%d body=%s", register.Code, register.Body.String())
	}
	if _, err := redis.GetVerificationCode(ctx, "verify@test.com", "register"); err == nil {
		t.Fatal("register verification code should be deleted after use")
	}

	user := testutil.CreateUser(t, db, "reset@test.com", "OldPassword123!", "ResetUser", false)
	login := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "OldPassword123!"})
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}
	sendResetMissing := doJSON(t, h, "POST", "/v2/auth/verification-code", map[string]any{"email": "missing-reset@test.com", "type": "reset"})
	if sendResetMissing.Code != 200 || parseJSON(t, sendResetMissing)["ttl"] != float64(0) {
		t.Fatalf("missing reset target should return ok ttl=0, got %d %s", sendResetMissing.Code, sendResetMissing.Body.String())
	}
	sendReset := doJSON(t, h, "POST", "/v2/auth/verification-code", map[string]any{"email": user.Email, "type": "reset"})
	if sendReset.Code != 200 {
		t.Fatalf("send reset status=%d body=%s", sendReset.Code, sendReset.Body.String())
	}
	resetCode, err := redis.GetVerificationCode(ctx, user.Email, "reset")
	if err != nil {
		t.Fatalf("reset code missing err=%v", err)
	}
	reset := doJSON(t, h, "POST", "/v2/auth/password/reset", map[string]any{"email": user.Email, "password": "NewPassword456!", "code": resetCode})
	if reset.Code != http.StatusNoContent || reset.Body.Len() != 0 {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	updated, _ := db.Users.GetByID(ctx, user.ID)
	if !util.VerifyPassword("NewPassword456!", updated.Password) {
		t.Fatal("reset password did not update password")
	}
	reuseRefresh := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, refresh)
	if reuseRefresh.Code != 401 {
		t.Fatalf("old refresh should be revoked after reset, got %d", reuseRefresh.Code)
	}
	if _, err := redis.GetVerificationCode(ctx, user.Email, "reset"); err == nil {
		t.Fatal("reset verification code should be deleted after use")
	}
}
