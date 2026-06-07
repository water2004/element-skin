package integration_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestSettingsVerificationAndFallbackModules(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	val, err := db.GetSetting(ctx, "enable_skin_library", "")
	if err != nil || val != "true" {
		t.Fatalf("enable_skin_library=%q err=%v", val, err)
	}
	missing, _ := db.GetSetting(ctx, "non_existent_key", "default_val")
	if missing != "default_val" {
		t.Fatalf("default setting mismatch: %q", missing)
	}
	if err := db.SetSetting(ctx, "test_key", "test_value"); err != nil {
		t.Fatal(err)
	}
	all, err := db.GetAllSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if all["test_key"] != "test_value" || all["enable_skin_library"] == "" {
		t.Fatalf("unexpected all settings: %#v", all)
	}

	if err := db.CreateVerificationCode(ctx, "test@verify.com", "ABCDEFGH", "register", 1); err != nil {
		t.Fatal(err)
	}
	code, expiresAt, ok, err := db.GetVerificationCode(ctx, "test@verify.com", "register")
	if err != nil || !ok || code != "ABCDEFGH" {
		t.Fatalf("verification code got=%q ok=%v err=%v", code, ok, err)
	}
	if expiresAt <= time.Now().UnixMilli() {
		t.Fatal("verification code should expire in the future")
	}
	if err := db.DeleteVerificationCode(ctx, "test@verify.com", "register"); err != nil {
		t.Fatal(err)
	}
	_, _, ok, _ = db.GetVerificationCode(ctx, "test@verify.com", "register")
	if ok {
		t.Fatal("verification code should be deleted")
	}

	eps, err := db.ListFallbackEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 0 {
		t.Fatalf("default fallback endpoints should be empty: %#v", eps)
	}
	if err := db.SaveFallbackEndpoints(ctx, []database.FallbackEndpoint{{
		Priority: 1, SessionURL: "s1", AccountURL: "a1", ServicesURL: "v1",
		CacheTTL: 60, SkinDomains: "d1,d2", EnableProfile: true, EnableHasJoined: true,
		EnableWhitelist: true, Note: "CustomEP",
	}}); err != nil {
		t.Fatal(err)
	}
	eps, err = db.ListFallbackEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0]["note"] != "CustomEP" {
		t.Fatalf("unexpected fallback endpoints: %#v", eps)
	}
	endpointID := eps[0]["id"].(int)
	domains, err := db.CollectFallbackSkinDomains(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 2 || domains[0] != "d1" || domains[1] != "d2" {
		t.Fatalf("unexpected domains: %#v", domains)
	}
	primary, err := db.GetPrimaryFallbackEndpoint(ctx)
	if err != nil || primary["note"] != "CustomEP" {
		t.Fatalf("unexpected primary: %#v err=%v", primary, err)
	}
	if err := db.AddWhitelistUser(ctx, "WhitelistedPlayer", endpointID); err != nil {
		t.Fatal(err)
	}
	in, err := db.IsUserInWhitelist(ctx, "WhitelistedPlayer", endpointID)
	if err != nil || !in {
		t.Fatalf("expected whitelist hit: %v", err)
	}
	miss, _ := db.IsUserInWhitelist(ctx, "NonExistent", endpointID)
	if miss {
		t.Fatal("unexpected whitelist hit")
	}
	users, err := db.ListWhitelistUsers(ctx, endpointID)
	if err != nil || len(users) != 1 || users[0]["username"] != "WhitelistedPlayer" {
		t.Fatalf("unexpected whitelist users: %#v err=%v", users, err)
	}
	if err := db.RemoveWhitelistUser(ctx, "WhitelistedPlayer", endpointID); err != nil {
		t.Fatal(err)
	}
	in, _ = db.IsUserInWhitelist(ctx, "WhitelistedPlayer", endpointID)
	if in {
		t.Fatal("whitelist user should be removed")
	}
}
