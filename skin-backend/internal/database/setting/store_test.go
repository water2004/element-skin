package setting_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/setting"
	"element-skin/backend/internal/testutil"
)

func TestStoreSetGetIntGroupAndAllExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := setting.Store{Pool: db.Pool}
	if err := store.Set(ctx, "sub_bool", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "sub_int", "42"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, "sub_bool", "false"); err != nil || got != "true" {
		t.Fatalf("setting get mismatch: got=%q err=%v", got, err)
	}
	if got, err := store.Int(ctx, "sub_int", 7); err != nil || got != 42 {
		t.Fatalf("setting int mismatch: got=%d err=%v", got, err)
	}
	group, err := store.Group(ctx, map[string]string{"sub_bool": "false", "missing": "false"})
	if err != nil || group["sub_bool"] != true || group["missing"] != false {
		t.Fatalf("setting group mismatch: group=%#v err=%v", group, err)
	}
	all, err := store.All(ctx)
	if err != nil || all["sub_bool"] != "true" || all["sub_int"] != "42" {
		t.Fatalf("setting all mismatch: all=%#v err=%v", all, err)
	}
}
