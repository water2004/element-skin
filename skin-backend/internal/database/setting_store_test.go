package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestSettingStoreSetGetGroupAndAllExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.SetSetting(ctx, "bool_setting", true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "int_setting", "42"); err != nil {
		t.Fatal(err)
	}
	group, err := db.GetSettingsGroup(ctx, map[string]string{"bool_setting": "false", "missing_bool": "false", "int_setting": "0"})
	if err != nil {
		t.Fatal(err)
	}
	if group["bool_setting"] != true || group["missing_bool"] != false || group["int_setting"] != "42" {
		t.Fatalf("settings group coercion mismatch: %#v", group)
	}
	if n, err := db.SettingInt(ctx, "int_setting", 7); err != nil || n != 42 {
		t.Fatalf("SettingInt should parse stored value: n=%d err=%v", n, err)
	}
	all, err := db.GetAllSettings(ctx)
	if err != nil || all["bool_setting"] != "true" || all["int_setting"] != "42" {
		t.Fatalf("GetAllSettings mismatch: all=%#v err=%v", all, err)
	}
}
