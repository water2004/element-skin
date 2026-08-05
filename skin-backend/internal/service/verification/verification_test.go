package verification_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type failingSender struct {
	err     error
	calls   int
	to      string
	code    string
	purpose string
}

func (s *failingSender) SendVerificationCode(_ context.Context, to, code, purpose string) error {
	s.calls++
	s.to = to
	s.code = code
	s.purpose = purpose
	return s.err
}

var _ mailsvc.Sender = (*failingSender)(nil)

func TestSendEmailChangeStoresExactPurposeAfterDelivery(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	sender := &failingSender{}
	settings := settingssvc.Settings{DB: db, Redis: cache}
	svc := verificationsvc.Service{DB: db, Redis: cache, Settings: settings, Sender: sender}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", "90"); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SendEmailChange(ctx, "new-email@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result["ttl"] != 90 {
		t.Fatalf("result=%#v; want ok=true ttl=90", result)
	}
	if sender.calls != 1 || sender.to != "new-email@test.com" || sender.purpose != verificationsvc.PurposeEmailChange || len(sender.code) != 8 {
		t.Fatalf("delivery mismatch: sender=%#v", sender)
	}
	stored, err := cache.GetVerificationCode(ctx, sender.to, sender.purpose)
	if err != nil || stored != sender.code {
		t.Fatalf("stored code=%q err=%v; want %q", stored, err, sender.code)
	}
}

func TestDeliveryFailureRemovesUnusableVerificationCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	sender := &failingSender{err: errors.New("SMTP delivery failed")}
	settings := settingssvc.Settings{DB: db, Redis: cache}
	svc := verificationsvc.Service{DB: db, Redis: cache, Settings: settings, Sender: sender}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}

	result, err := svc.SendEmailChange(ctx, "delivery-failure@test.com")
	if result != nil || err == nil || err.Error() != "SMTP delivery failed" {
		t.Fatalf("result=%#v err=%v; want nil exact SMTP failure", result, err)
	}
	if sender.calls != 1 || sender.to != "delivery-failure@test.com" || sender.purpose != verificationsvc.PurposeEmailChange {
		t.Fatalf("failed delivery call mismatch: %#v", sender)
	}
	if _, err := cache.GetVerificationCode(ctx, sender.to, sender.purpose); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed delivery left verification code: %v", err)
	}
}

func TestSendEmailChangeRejectsDisabledAndMissingSenderExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	settings := settingssvc.Settings{DB: db, Redis: cache}
	svc := verificationsvc.Service{DB: db, Redis: cache, Settings: settings}

	if result, err := svc.SendEmailChange(ctx, "disabled@test.com"); result != nil || !httpErrorIs(err, 400, "Email verification is disabled") {
		t.Fatalf("disabled result=%#v err=%#v", result, err)
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if result, err := svc.SendEmailChange(ctx, "missing-sender@test.com"); result != nil || err == nil || err.Error() != "verification email sender is not configured" {
		t.Fatalf("missing sender result=%#v err=%v", result, err)
	}
}

func httpErrorIs(err error, status int, detail string) bool {
	var target util.HTTPError
	if !errors.As(err, &target) {
		return false
	}
	return target.Status == status && target.Detail == detail
}
