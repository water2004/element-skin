package site_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestVerificationSendAndVerifyExactStoredCode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := site.Site{DB: db, Cfg: testutil.TestConfig()}
	if err := db.SetSetting(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "email_verify_ttl", "180"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SendVerificationCode(ctx, "verify-service@test.com", "register")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true || res["ttl"] != 180 {
		t.Fatalf("verification response mismatch: %#v", res)
	}
	code, expiresAt, ok, err := db.GetVerificationCode(ctx, "verify-service@test.com", "register")
	if err != nil || !ok || len(code) != 8 || strings.ToUpper(code) != code || expiresAt <= database.NowMS() {
		t.Fatalf("stored verification code mismatch: code=%q expires=%d ok=%v err=%v", code, expiresAt, ok, err)
	}
	verified, err := svc.VerifyCode(ctx, "verify-service@test.com", strings.ToLower(code), "register")
	if err != nil || !verified {
		t.Fatalf("VerifyCode should be case-insensitive: verified=%v err=%v", verified, err)
	}
}
