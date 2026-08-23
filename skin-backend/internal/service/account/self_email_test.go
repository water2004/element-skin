package account_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/testutil"
)

type recordedVerificationEmail struct {
	recipient string
	code      string
	purpose   string
}

type recordingAccountMailSender struct {
	messages []recordedVerificationEmail
	err      error
}

func (s *recordingAccountMailSender) SendVerificationCode(_ context.Context, recipient, code, purpose string) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, recordedVerificationEmail{recipient: recipient, code: code, purpose: purpose})
	return nil
}

var _ mailsvc.Sender = (*recordingAccountMailSender)(nil)

func TestEmailChangeSendsToNewAddressAndConsumesCodeExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	sender := &recordingAccountMailSender{}
	settings := settingssvc.Settings{DB: db, Redis: cache}
	verification := verificationsvc.Service{DB: db, Redis: cache, Settings: settings, Sender: sender}
	svc := accountsvc.AccountService{DB: db, Redis: cache, Verification: verification}
	user := testutil.CreateUser(t, db, "email-change-old@test.com", "Password123", "EmailChange", false)
	actor := accountActor(t, db, user.ID)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", "180"); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: user.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SendEmailChangeCode(ctx, actor, "  email-change-new@test.com  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result["ttl"] != 180 {
		t.Fatalf("email verification response mismatch: %#v", result)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("mail count=%d; want exactly 1", len(sender.messages))
	}
	message := sender.messages[0]
	if message.recipient != "email-change-new@test.com" || message.purpose != verificationsvc.PurposeEmailChange || len(message.code) != 8 || strings.ToUpper(message.code) != message.code {
		t.Fatalf("verification mail mismatch: %#v", message)
	}
	stored, err := cache.GetVerificationCode(ctx, message.recipient, message.purpose)
	if err != nil || stored != message.code {
		t.Fatalf("stored code=%q err=%v; want %q", stored, err, message.code)
	}
	if err := svc.ChangeEmailSelf(ctx, actor, message.recipient, strings.ToLower(message.code)); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || updated == nil || updated.Email != message.recipient {
		t.Fatalf("updated user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetVerificationCode(ctx, message.recipient, message.purpose); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful change must consume code, got %v", err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful change must invalidate auth cache, got %v", err)
	}
}

func TestEmailChangeRejectsInvalidTargetsAndCodesWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountServiceWithVerification(db, cache)
	user := testutil.CreateUser(t, db, "email-change-invalid@test.com", "Password123", "EmailChangeInvalid", false)
	other := testutil.CreateUser(t, db, "email-change-used@test.com", "Password123", "EmailChangeUsed", false)
	actor := accountActor(t, db, user.ID)

	for _, tc := range []struct {
		name   string
		email  string
		status int
		detail string
	}{
		{name: "invalid", email: "not-an-email", status: http.StatusBadRequest, detail: "email.validate.invalid"},
		{name: "current", email: strings.ToUpper(user.Email), status: http.StatusBadRequest, detail: "email.update.conflict"},
		{name: "used", email: other.Email, status: http.StatusBadRequest, detail: "email.register.already_exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.SendEmailChangeCode(ctx, actor, tc.email); !httpErrorIs(err, tc.status, tc.detail) {
				t.Fatalf("SendEmailChangeCode error=%#v; want %d %q", err, tc.status, tc.detail)
			}
			if err := svc.ChangeEmailSelf(ctx, actor, tc.email, "EMAIL123"); !httpErrorIs(err, tc.status, tc.detail) {
				t.Fatalf("ChangeEmailSelf error=%#v; want %d %q", err, tc.status, tc.detail)
			}
		})
	}

	const target = "email-change-code@test.com"
	const code = "EMAIL123"
	if err := cache.SetVerificationCode(ctx, target, verificationsvc.PurposeEmailChange, code, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangeEmailSelf(ctx, actor, target, "WRONG123"); !httpErrorIs(err, http.StatusBadRequest, "verification_code.verify.invalid") {
		t.Fatalf("wrong code error=%#v", err)
	}
	if stored, err := cache.GetVerificationCode(ctx, target, verificationsvc.PurposeEmailChange); err != nil || stored != code {
		t.Fatalf("wrong code must remain usable: code=%q err=%v", stored, err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email {
		t.Fatalf("rejected changes mutated user: user=%#v err=%v", unchanged, err)
	}
}

func TestEmailChangeRestoresCodeWhenDatabaseWriteFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountServiceWithVerification(db, cache)
	user := testutil.CreateUser(t, db, "email-change-db-failure@test.com", "Password123", "EmailChangeDBFailure", false)
	actor := accountActor(t, db, user.ID)
	const target = "email-change-db-target@test.com"
	const code = "EMAILDB1"
	if err := cache.SetVerificationCode(ctx, target, verificationsvc.PurposeEmailChange, code, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_email_change() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'email update rejected' USING ERRCODE = '23514';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_email_change_trigger BEFORE UPDATE OF email ON users
		FOR EACH ROW EXECUTE FUNCTION reject_email_change()
	`); err != nil {
		t.Fatal(err)
	}

	err := svc.ChangeEmailSelf(ctx, actor, target, code)
	if err == nil {
		t.Fatal("database rejection should be returned")
	}
	if stored, restoreErr := cache.GetVerificationCode(ctx, target, verificationsvc.PurposeEmailChange); restoreErr != nil || stored != code {
		t.Fatalf("database failure must restore code: code=%q err=%v", stored, restoreErr)
	}
	unchanged, getErr := db.Users.GetByID(ctx, user.ID)
	if getErr != nil || unchanged == nil || unchanged.Email != user.Email {
		t.Fatalf("database failure mutated email: user=%#v err=%v", unchanged, getErr)
	}
}
