package verification_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database/verification"
	"element-skin/backend/internal/testutil"
)

func TestStoreCreateGetUpsertAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := verification.Store{Pool: db.Pool}
	if err := store.CreateCode(ctx, "sub-verify@test.com", "ABC123", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, expiresAt, ok, err := store.GetCode(ctx, "sub-verify@test.com", "register")
	if err != nil || !ok || code != "ABC123" || expiresAt <= time.Now().UnixMilli() {
		t.Fatalf("verification get mismatch: code=%q expires=%d ok=%v err=%v", code, expiresAt, ok, err)
	}
	if err := store.CreateCode(ctx, "sub-verify@test.com", "XYZ789", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, _, ok, err = store.GetCode(ctx, "sub-verify@test.com", "register")
	if err != nil || !ok || code != "XYZ789" {
		t.Fatalf("verification upsert mismatch: code=%q ok=%v err=%v", code, ok, err)
	}
	if err := store.DeleteCode(ctx, "sub-verify@test.com", "register"); err != nil {
		t.Fatal(err)
	}
	if code, _, ok, err := store.GetCode(ctx, "sub-verify@test.com", "register"); err != nil || ok || code != "" {
		t.Fatalf("verification should be deleted: code=%q ok=%v err=%v", code, ok, err)
	}
}
