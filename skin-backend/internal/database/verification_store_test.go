package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestVerificationStoreUpsertGetAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.CreateVerificationCode(ctx, "verify-store@test.com", "123456", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, expiresAt, ok, err := db.GetVerificationCode(ctx, "verify-store@test.com", "register")
	if err != nil || !ok || code != "123456" || expiresAt <= database.NowMS() {
		t.Fatalf("verification code mismatch: code=%q expires=%d ok=%v err=%v", code, expiresAt, ok, err)
	}
	if err := db.CreateVerificationCode(ctx, "verify-store@test.com", "654321", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, _, ok, err = db.GetVerificationCode(ctx, "verify-store@test.com", "register")
	if err != nil || !ok || code != "654321" {
		t.Fatalf("verification upsert should replace code: code=%q ok=%v err=%v", code, ok, err)
	}
	if err := db.DeleteVerificationCode(ctx, "verify-store@test.com", "register"); err != nil {
		t.Fatal(err)
	}
	if code, _, ok, err := db.GetVerificationCode(ctx, "verify-store@test.com", "register"); err != nil || ok || code != "" {
		t.Fatalf("verification code should be deleted: code=%q ok=%v err=%v", code, ok, err)
	}
}
