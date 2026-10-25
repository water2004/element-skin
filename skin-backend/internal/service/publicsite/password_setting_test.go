package publicsite_test

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	publicsvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSecurityPolicyToggleStaysPrivateAndPreservesPublicSettings(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	cache := redisstore.NewMemoryStore()
	settings := settingssvc.Settings{DB: db, Redis: cache}
	svc := publicsvc.Service{DB: db, Redis: cache, Settings: settings, SiteURL: "http://test", APIURL: "http://test/api", CacheTTL: time.Minute}
	initial, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := initial["enable_strong_password_check"]; exposed {
		t.Fatalf("public settings exposed the password policy: %#v", initial)
	}
	before, err := cache.GetPublicSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		if err := settings.SaveGroupAndInvalidate(ctx, "security", map[string]any{"enable_strong_password_check": enabled}); err != nil {
			t.Fatal(err)
		}
		private, err := settings.Get(ctx, "enable_strong_password_check", "false")
		if err != nil || private != strconv.FormatBool(enabled) {
			t.Fatalf("private password setting=%q; want %t err=%v", private, enabled, err)
		}
		updated, err := svc.PublicSettings(ctx, permission.GuestActor())
		if err != nil || !reflect.DeepEqual(updated, before) {
			t.Fatalf("private security update changed public settings: got=%#v err=%v", updated, err)
		}
		cached, err := cache.GetPublicSettings(ctx)
		if err != nil || !reflect.DeepEqual(cached, before) {
			t.Fatalf("private security update changed public cache: got=%#v err=%v", cached, err)
		}
	}
}
