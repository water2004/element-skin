package auth_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestVerificationSendAndVerifyExactStoredCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
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
	if len(res) != 1 || res["ttl"] != 180 {
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
	svc := newAuthService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "verify-existing@test.com", "Password123", "VerifyExisting", false)

	if _, err := svc.SendVerificationCode(ctx, "verify-new@test.com", "register"); !httpError(err, 400, "email_verification.create.disabled") {
		t.Fatalf("disabled verification should reject exactly, got %#v", err)
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SendVerificationCode(ctx, "missing-reset@test.com", "reset")
	if err != nil || len(res) != 1 || res["ttl"] != 0 {
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
		{"bad email", "not-an-email", "register", "email.validate.invalid"},
		{"registered email", "verify-existing@test.com", "register", "email.register.already_exists"},
		{"bad type", "verify-new@test.com", "bad", "verification_type.validate.invalid"},
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

type verificationCodeFailStore struct {
	redisstore.Store
	failSet     bool
	failGet     bool
	failConsume bool
}

func (s *verificationCodeFailStore) SetVerificationCode(ctx context.Context, email, typ, code string, ttl time.Duration) error {
	if s.failSet {
		return errors.New("set verification failed")
	}
	return s.Store.SetVerificationCode(ctx, email, typ, code, ttl)
}

func (s *verificationCodeFailStore) GetVerificationCode(ctx context.Context, email, typ string) (string, error) {
	if s.failGet {
		return "", errors.New("get verification failed")
	}
	return s.Store.GetVerificationCode(ctx, email, typ)
}

func (s *verificationCodeFailStore) ConsumeVerificationCode(ctx context.Context, email, typ, code string) (bool, error) {
	if s.failConsume {
		return false, errors.New("consume verification failed")
	}
	return s.Store.ConsumeVerificationCode(ctx, email, typ, code)
}

func TestVerificationPropagatesRedisErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "verify-redis-error@test.com", "Password123", "VerifyRedisError", false)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}

	setFail := &verificationCodeFailStore{Store: testutil.NewMemoryRedis(), failSet: true}
	svc := newAuthServiceWithRedis(db, testutil.TestConfig(), setFail)
	if _, err := svc.SendVerificationCode(ctx, "verify-default-type@test.com", ""); err == nil || err.Error() != "set verification failed" {
		t.Fatalf("SendVerificationCode Redis set error=%v; want exact set failure", err)
	}
	if _, err := svc.Redis.GetVerificationCode(ctx, "verify-default-type@test.com", "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed send must not store default register code, got %v", err)
	}

	getFail := &verificationCodeFailStore{Store: testutil.NewMemoryRedis(), failGet: true}
	svc = newAuthServiceWithRedis(db, testutil.TestConfig(), getFail)
	ok, err := svc.VerifyCode(ctx, user.Email, "RESETERR", "reset")
	if ok || err == nil || err.Error() != "get verification failed" {
		t.Fatalf("VerifyCode Redis get error mismatch: ok=%v err=%v", ok, err)
	}
	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "RESETERR"); err == nil || err.Error() != "get verification failed" {
		t.Fatalf("ResetPassword VerifyCode Redis error=%v; want exact get failure", err)
	}

	consumeFail := &verificationCodeFailStore{Store: testutil.NewMemoryRedis(), failConsume: true}
	svc = newAuthServiceWithRedis(db, testutil.TestConfig(), consumeFail)
	if err := consumeFail.Store.SetVerificationCode(ctx, user.Email, "reset", "RESETERR", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "RESETERR"); err == nil || err.Error() != "consume verification failed" {
		t.Fatalf("ResetPassword consume Redis error=%v; want exact consume failure", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("consume failure must preserve password: user=%#v err=%v", unchanged, err)
	}
}

func TestResetPasswordRejectsDisabledWeakAndBadCodesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "verify-reset@test.com", "Password123", "VerifyReset", false)

	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "NO_CODE"); !httpError(err, 403, "password_reset.create.disabled") {
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
	if err := svc.ResetPassword(ctx, user.Email, "NewPassword123", "WRONG"); !httpError(err, 400, "verification_code.verify.invalid") {
		t.Fatalf("bad reset code should reject exactly, got %#v", err)
	}
}

func TestResetPasswordMissingAccountPreservesVerificationCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
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
	if !httpError(err, 404, "user.resolve.not_found") {
		t.Fatalf("missing reset account should reject exactly, got %#v", err)
	}
	stored, err := svc.Redis.GetVerificationCode(ctx, email, "reset")
	if err != nil || stored != code {
		t.Fatalf("failed reset must preserve code for its remaining TTL: code=%q err=%v", stored, err)
	}
}

func TestResetPasswordPreservesCredentialsRefreshAndCodeWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "reset-ygg-fail@test.com", "Password123", "ResetYggFail", false)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	const code = "RESETYGG"
	const refreshHash = "reset_ygg_fail_refresh"
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := newAuthServiceWithRedis(db, testutil.TestConfig(), cache)
	if err := cache.SetVerificationCode(ctx, user.Email, "reset", code, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}

	err := svc.ResetPassword(ctx, user.Email, "NewPassword123", code)
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("ygg revocation failure should be returned exactly, got %v", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("NewPassword123", unchanged.Password) {
		t.Fatalf("failed reset must preserve old password: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh == nil || refresh["user_id"] != user.ID {
		t.Fatalf("failed reset must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if stored, err := cache.GetVerificationCode(ctx, user.Email, "reset"); err != nil || stored != code {
		t.Fatalf("failed reset must preserve verification code: code=%q err=%v", stored, err)
	}
}

func TestResetPasswordConcurrentCodeAllowsExactlyOneSuccess(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "reset-once@test.com", "Password123", "ResetOnce", false)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	const code = "RESETONCE"
	if err := svc.Redis.SetVerificationCode(ctx, user.Email, "reset", code, time.Hour); err != nil {
		t.Fatal(err)
	}

	type result struct {
		password string
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, password := range []string{"ConcurrentPassword123", "OtherConcurrentPassword123"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- result{password: password, err: svc.ResetPassword(ctx, user.Email, password, code)}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winner string
	failures := 0
	for result := range results {
		if result.err == nil {
			if winner != "" {
				t.Fatalf("multiple password resets succeeded: %q and %q", winner, result.password)
			}
			winner = result.password
			continue
		}
		if !httpError(result.err, 400, "verification_code.verify.invalid") {
			t.Fatalf("losing reset returned unexpected error: %#v", result.err)
		}
		failures++
	}
	if winner == "" || failures != 1 {
		t.Fatalf("reset outcomes: winner=%q failures=%d, want one success and one exact rejection", winner, failures)
	}
	updated, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || updated == nil || !util.VerifyPassword(winner, updated.Password) {
		t.Fatalf("stored password does not match sole winner %q: user=%#v err=%v", winner, updated, err)
	}
	for _, password := range []string{"ConcurrentPassword123", "OtherConcurrentPassword123"} {
		if password != winner && util.VerifyPassword(password, updated.Password) {
			t.Fatalf("stored password unexpectedly matches losing reset %q", password)
		}
	}
	if _, err := svc.Redis.GetVerificationCode(ctx, user.Email, "reset"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful reset must consume code, got %v", err)
	}
}

func TestResetPasswordRestoresCodeWhenDatabaseUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "reset-db-fail@test.com", "Password123", "ResetDBFail", false)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", "180"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	const code = "RESETDB1"
	const refreshHash = "reset_db_failure_refresh"
	if err := svc.Redis.SetVerificationCode(ctx, user.Email, "reset", code, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_test_password_update() RETURNS trigger AS $$
		BEGIN
			IF NEW.password <> OLD.password THEN
				RAISE EXCEPTION 'test password update rejected'
					USING ERRCODE = '23514', CONSTRAINT = 'users_password_reset_guard';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_test_password_update
		BEFORE UPDATE OF password ON users
		FOR EACH ROW EXECUTE FUNCTION reject_test_password_update()
	`); err != nil {
		t.Fatal(err)
	}

	err := svc.ResetPassword(ctx, user.Email, "NewPassword123", code)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "users_password_reset_guard" {
		t.Fatalf("password update error=%#v, want exact users_password_reset_guard violation", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) ||
		util.VerifyPassword("NewPassword123", unchanged.Password) {
		t.Fatalf("failed reset changed password: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh == nil || refresh["user_id"] != user.ID {
		t.Fatalf("failed reset changed refresh token: refresh=%#v err=%v", refresh, err)
	}
	if stored, err := svc.Redis.GetVerificationCode(ctx, user.Email, "reset"); err != nil || stored != code {
		t.Fatalf("failed reset did not restore exact code: code=%q err=%v", stored, err)
	}
}
