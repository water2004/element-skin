package settings_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSettingsSaveGetRoundTripExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}

	if err := settings.SaveGroup(ctx, "site", map[string]any{
		"site_name":           "Exact Skin",
		"allow_register":      false,
		"max_texture_size":    2048,
		"profile_uuid_mode":   "offline",
		"site_url":            "skin.example.com/root/",
		"api_url":             "https://api.example.com/skinapi/",
		"unknown_should_skip": "x",
	}); err != nil {
		t.Fatal(err)
	}
	site, err := settings.GetGroup(ctx, "site")
	if err != nil {
		t.Fatal(err)
	}
	if site["site_name"] != "Exact Skin" || site["allow_register"] != false || site["max_texture_size"] != 2048 ||
		site["profile_uuid_mode"] != "offline" || site["site_url"] != "skin.example.com/root/" || site["api_url"] != "https://api.example.com/skinapi/" {
		t.Fatalf("unexpected site settings: %#v", site)
	}
	if raw, _ := db.Settings.Get(ctx, "unknown_should_skip", "missing"); raw != "missing" {
		t.Fatalf("unknown setting should not persist, got %q", raw)
	}

	if err := db.Settings.Set(ctx, "smtp_password", "existing-secret"); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "email", map[string]any{"smtp_host": "mail.example.com", "smtp_password": ""}); err != nil {
		t.Fatal(err)
	}
	email, err := settings.GetGroup(ctx, "email")
	if err != nil {
		t.Fatal(err)
	}
	if email["smtp_host"] != "mail.example.com" || email["smtp_password"] != "existing-secret" || email["smtp_ssl"] != true || email["smtp_port"] != 465 {
		t.Fatalf("unexpected email settings: %#v", email)
	}

	if err := settings.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"priority":         1,
			"session_url":      "https://session.example",
			"account_url":      "https://account.example",
			"services_url":     "https://services.example",
			"cache_ttl":        30,
			"skin_domains":     []any{"skins.example", "cdn.example"},
			"enable_profile":   true,
			"enable_hasjoined": false,
			"enable_whitelist": true,
			"note":             "primary",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	fallback, err := settings.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if fallback["fallback_strategy"] != "parallel" {
		t.Fatalf("fallback strategy did not persist: %#v", fallback)
	}
	fallbacks := fallback["fallbacks"].([]map[string]any)
	if len(fallbacks) != 1 || fallbacks[0]["session_url"] != "https://session.example" || fallbacks[0]["enable_hasjoined"] != false {
		t.Fatalf("unexpected fallback endpoints: %#v", fallbacks)
	}

	if err := settings.SaveGroup(ctx, "easter_eggs", map[string]any{
		"easter_eggs_enabled": []any{"april-fools", "christmas", "dragon-boat", "april-fools"},
	}); err != nil {
		t.Fatal(err)
	}
	easterEggs, err := settings.GetGroup(ctx, "easter_eggs")
	if err != nil {
		t.Fatal(err)
	}
	enabled := easterEggs["easter_eggs_enabled"].([]string)
	if len(enabled) != 3 || enabled[0] != "april-fools" || enabled[1] != "christmas" || enabled[2] != "dragon-boat" {
		t.Fatalf("unexpected easter egg settings: %#v", easterEggs)
	}
}

func TestSettingsRejectInvalidGroupAndProfileMode(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if _, err := settings.GetGroup(context.Background(), "missing"); err == nil {
		t.Fatal("missing settings group should reject")
	}
	if err := settings.SaveGroup(context.Background(), "site", map[string]any{"profile_uuid_mode": "bad"}); err == nil {
		t.Fatal("invalid profile_uuid_mode should reject")
	}
	if err := settings.SaveGroup(context.Background(), "easter_eggs", map[string]any{"easter_eggs_enabled": []any{"missing"}}); err == nil {
		t.Fatal("invalid easter egg should reject")
	}
}

func TestSettingsInvalidFallbackGroupPreservesExistingConfiguration(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "serial",
		"fallbacks": []any{map[string]any{
			"priority":         7,
			"session_url":      "https://old-session.example",
			"account_url":      "https://old-account.example",
			"services_url":     "https://old-services.example",
			"cache_ttl":        45,
			"skin_domains":     []any{"old-skins.example"},
			"enable_profile":   false,
			"enable_hasjoined": true,
			"enable_whitelist": true,
			"note":             "existing",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"session_url":  "https://new-session.example",
			"account_url":  "",
			"services_url": "https://new-services.example",
		}},
	})
	httpErr, ok := err.(util.HTTPError)
	if !ok || httpErr.Status != 400 || httpErr.Error() != "fallback_url.validate.required" {
		t.Fatalf("invalid fallback error = %#v, want exact 400 validation error", err)
	}

	strategy, err := db.Settings.Get(ctx, "fallback_strategy", "")
	if err != nil || strategy != "serial" {
		t.Fatalf("failed fallback save changed strategy: strategy=%q err=%v", strategy, err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("failed fallback save changed endpoint count: %#v", endpoints)
	}
	got := endpoints[0]
	if got["priority"] != 7 ||
		got["session_url"] != "https://old-session.example" ||
		got["account_url"] != "https://old-account.example" ||
		got["services_url"] != "https://old-services.example" ||
		got["cache_ttl"] != 45 ||
		!reflect.DeepEqual(got["skin_domains"], []string{"old-skins.example"}) ||
		got["enable_profile"] != false ||
		got["enable_hasjoined"] != true ||
		got["enable_whitelist"] != true ||
		got["note"] != "existing" {
		t.Fatalf("failed fallback save changed existing endpoint: %#v", got)
	}
}

func TestSettingsFallbackDatabaseFailureRollsBackStrategyAndEndpoints(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "serial",
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://stable-session.example",
			"account_url":  "https://stable-account.example",
			"services_url": "https://stable-services.example",
			"cache_ttl":    60,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE fallback_endpoints ADD CONSTRAINT reject_parallel_endpoint CHECK (note <> 'reject')`,
	); err != nil {
		t.Fatal(err)
	}

	err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"priority":     2,
			"session_url":  "https://rejected-session.example",
			"account_url":  "https://rejected-account.example",
			"services_url": "https://rejected-services.example",
			"cache_ttl":    30,
			"note":         "reject",
		}},
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("fallback database failure = %#v, want PostgreSQL 23514", err)
	}
	strategy, err := db.Settings.Get(ctx, "fallback_strategy", "")
	if err != nil || strategy != "serial" {
		t.Fatalf("failed transaction changed strategy: strategy=%q err=%v", strategy, err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 ||
		endpoints[0]["priority"] != 1 ||
		endpoints[0]["session_url"] != "https://stable-session.example" ||
		endpoints[0]["account_url"] != "https://stable-account.example" ||
		endpoints[0]["services_url"] != "https://stable-services.example" ||
		endpoints[0]["cache_ttl"] != 60 {
		t.Fatalf("failed transaction changed endpoints: %#v", endpoints)
	}
}

func TestSettingsFallbackProbeIntervalValidationAndPersistence(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	redis := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: redis}

	defaults, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if defaults["fallback_probe_interval"] != 600 {
		t.Fatalf("default probe interval should be 600 seconds, got %#v", defaults["fallback_probe_interval"])
	}

	if err := svc.SaveGroup(ctx, "fallback", map[string]any{"fallback_probe_interval": 1800}); err != nil {
		t.Fatalf("valid probe interval should persist: %v", err)
	}
	if err := redis.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if updated["fallback_probe_interval"] != 1800 {
		t.Fatalf("probe interval did not persist: %#v", updated["fallback_probe_interval"])
	}

	cases := []any{59, 86401, "abc", -10}
	for _, value := range cases {
		err := svc.SaveGroup(ctx, "fallback", map[string]any{"fallback_probe_interval": value})
		httpErr, ok := err.(util.HTTPError)
		if !ok || httpErr.Status != 400 {
			t.Fatalf("invalid probe interval %v should return HTTP 400, got %#v", value, err)
		}
	}
	if err := redis.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	preserved, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if preserved["fallback_probe_interval"] != 1800 {
		t.Fatalf("invalid attempts changed persisted interval: %#v", preserved["fallback_probe_interval"])
	}
}

func TestSettingsReadUpdateGroupPermissionsAndCacheInvalidationExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	redis := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: redis}
	readActor := settingsActor("site_settings.read.any")
	updateActor := settingsActor("site_settings.update.any")

	if err := redis.SetSetting(ctx, "site_name", "Cached Site", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetPublicSettings(ctx, map[string]any{"site_name": "Cached Site"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateGroup(ctx, updateActor, "site", map[string]any{"site_name": "Updated Site", "allow_register": false}); err != nil {
		t.Fatal(err)
	}
	if cached, err := redis.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) || cached != "" {
		t.Fatalf("UpdateGroup should invalidate setting cache: cached=%q err=%v", cached, err)
	}
	if cached, err := redis.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) || cached != nil {
		t.Fatalf("UpdateGroup should invalidate public settings cache: cached=%#v err=%v", cached, err)
	}

	site, err := svc.ReadGroup(ctx, readActor, "site")
	if err != nil {
		t.Fatal(err)
	}
	if site["site_name"] != "Updated Site" || site["allow_register"] != false {
		t.Fatalf("ReadGroup site settings mismatch: %#v", site)
	}

	if _, err := svc.ReadGroup(ctx, permission.Actor{}, "site"); !settingsHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ReadGroup without permission mismatch: %#v", err)
	}
	if err := svc.UpdateGroup(ctx, permission.Actor{}, "site", map[string]any{"site_name": "Denied"}); !settingsHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UpdateGroup without permission mismatch: %#v", err)
	}
	if err := svc.UpdateGroup(ctx, updateActor, "missing", map[string]any{"site_name": "Denied"}); !settingsHTTPError(err, http.StatusBadRequest, "settings_group.validate.invalid") {
		t.Fatalf("UpdateGroup invalid group mismatch: %#v", err)
	}
}

func TestSettingsFallbackUpdateInvalidatesPublicKeyCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	redis := testutil.NewMemoryRedis()
	svc := settings.Settings{DB: db, Redis: redis}
	keys := model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: "old-profile"}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: "old-certificate"}},
	}
	if err := redis.SetFallbackPublicKeys(ctx, "old-source", keys, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateGroup(ctx, settingsActor("site_settings.update.any"), "fallback", map[string]any{
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://new.example/session",
			"account_url":  "https://new.example/account",
			"services_url": "https://new.example/services",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := redis.GetFallbackPublicKeys(ctx, []string{"old-source"})
	if err != nil || len(got) != 0 {
		t.Fatalf("fallback update cache result=%#v err=%v, want empty cache", got, err)
	}
}

func TestSettingsFallbackUpdateRejectsUnknownStableIDWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "serial",
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://stable.example/session",
			"account_url":  "https://stable.example/account",
			"services_url": "https://stable.example/services",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(before) != 1 {
		t.Fatalf("initial endpoints=%#v err=%v; want exactly one", before, err)
	}
	err = svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallback_strategy": "parallel",
		"fallbacks": []any{map[string]any{
			"id":           float64(before[0]["id"].(int) + 1000),
			"priority":     1,
			"session_url":  "https://missing.example/session",
			"account_url":  "https://missing.example/account",
			"services_url": "https://missing.example/services",
		}},
	})
	if !settingsHTTPError(err, http.StatusBadRequest, "fallback_endpoint.resolve.not_found") {
		t.Fatalf("unknown stable endpoint error=%#v; want exact HTTP 400", err)
	}
	after, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("rejected stable ID changed endpoints: after=%#v before=%#v err=%v", after, before, err)
	}
	strategy, err := db.Settings.Get(ctx, "fallback_strategy", "")
	if err != nil || strategy != "serial" {
		t.Fatalf("rejected stable ID changed strategy: strategy=%q err=%v", strategy, err)
	}
}

func TestSettingsFallbackSavePreservesIndependentEndpointStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	initialPayload := map[string]any{
		"fallback_strategy":       "serial",
		"fallback_probe_interval": 600,
		"fallbacks": []any{
			map[string]any{"priority": 1, "session_url": "https://one.example/session", "account_url": "https://one.example/account", "services_url": "https://one.example/services", "cache_ttl": 60, "skin_domains": []any{"one.example"}, "enable_profile": true, "enable_hasjoined": true, "enable_whitelist": true, "note": "one"},
			map[string]any{"priority": 2, "session_url": "https://two.example/session", "account_url": "https://two.example/account", "services_url": "https://two.example/services", "cache_ttl": 120, "skin_domains": []any{"two.example"}, "enable_profile": false, "enable_hasjoined": true, "enable_whitelist": true, "note": "two"},
			map[string]any{"priority": 3, "session_url": "https://remove.example/session", "account_url": "https://remove.example/account", "services_url": "https://remove.example/services", "cache_ttl": 300, "skin_domains": []any{"remove.example"}, "enable_profile": true, "enable_hasjoined": false, "enable_whitelist": true, "note": "remove"},
		},
	}
	if err := svc.SaveGroup(ctx, "fallback", initialPayload); err != nil {
		t.Fatal(err)
	}
	initial, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(initial) != 3 {
		t.Fatalf("initial endpoints=%#v err=%v; want exactly three", initial, err)
	}
	oneID := initial[0]["id"].(int)
	twoID := initial[1]["id"].(int)
	removeID := initial[2]["id"].(int)
	for endpointID, username := range map[int]string{oneID: "OnePlayer", twoID: "TwoPlayer", removeID: "RemovedPlayer"} {
		if err := db.Fallbacks.AddWhitelistUser(ctx, username, endpointID); err != nil {
			t.Fatal(err)
		}
	}
	oneWhitelist, err := db.Fallbacks.ListWhitelistUsers(ctx, oneID)
	if err != nil || len(oneWhitelist) != 1 {
		t.Fatalf("endpoint one whitelist=%#v err=%v", oneWhitelist, err)
	}
	twoWhitelist, err := db.Fallbacks.ListWhitelistUsers(ctx, twoID)
	if err != nil || len(twoWhitelist) != 1 {
		t.Fatalf("endpoint two whitelist=%#v err=%v", twoWhitelist, err)
	}

	updatedPayload := map[string]any{
		"fallback_strategy":       "parallel",
		"fallback_probe_interval": 1800,
		"fallbacks": []any{
			map[string]any{"id": float64(twoID), "priority": 1, "session_url": "https://two.example/session-v2", "account_url": "https://two.example/account-v2", "services_url": "https://two.example/services-v2", "cache_ttl": 240, "skin_domains": []any{"two-v2.example", "cdn.two.example"}, "enable_profile": true, "enable_hasjoined": false, "enable_whitelist": true, "note": "two updated"},
			map[string]any{"id": nil, "priority": 2, "session_url": "https://new.example/session", "account_url": "https://new.example/account", "services_url": "https://new.example/services", "cache_ttl": 90, "skin_domains": []any{"new.example"}, "enable_profile": false, "enable_hasjoined": true, "enable_whitelist": false, "note": "new"},
			map[string]any{"id": float64(oneID), "priority": 3, "session_url": "https://one.example/session", "account_url": "https://one.example/account", "services_url": "https://one.example/services", "cache_ttl": 60, "skin_domains": []any{"one.example"}, "enable_profile": true, "enable_hasjoined": true, "enable_whitelist": true, "note": "one"},
		},
	}
	if err := svc.SaveGroup(ctx, "fallback", updatedPayload); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if updated["fallback_strategy"] != "parallel" || updated["fallback_probe_interval"] != 1800 {
		t.Fatalf("updated fallback settings mismatch: %#v", updated)
	}
	endpoints, ok := updated["fallbacks"].([]map[string]any)
	if !ok || len(endpoints) != 3 {
		t.Fatalf("updated fallback endpoints=%#v; want exactly three", updated["fallbacks"])
	}
	newID := endpoints[1]["id"].(int)
	if newID == oneID || newID == twoID || newID == removeID {
		t.Fatalf("new endpoint reused an existing identity: %#v", endpoints[1])
	}
	expectedEndpoints := []map[string]any{
		{"id": twoID, "priority": 1, "session_url": "https://two.example/session-v2", "account_url": "https://two.example/account-v2", "services_url": "https://two.example/services-v2", "cache_ttl": 240, "skin_domains": []string{"two-v2.example", "cdn.two.example"}, "enable_profile": true, "enable_hasjoined": false, "enable_whitelist": true, "note": "two updated"},
		{"id": newID, "priority": 2, "session_url": "https://new.example/session", "account_url": "https://new.example/account", "services_url": "https://new.example/services", "cache_ttl": 90, "skin_domains": []string{"new.example"}, "enable_profile": false, "enable_hasjoined": true, "enable_whitelist": false, "note": "new"},
		{"id": oneID, "priority": 3, "session_url": "https://one.example/session", "account_url": "https://one.example/account", "services_url": "https://one.example/services", "cache_ttl": 60, "skin_domains": []string{"one.example"}, "enable_profile": true, "enable_hasjoined": true, "enable_whitelist": true, "note": "one"},
	}
	if !reflect.DeepEqual(endpoints, expectedEndpoints) {
		t.Fatalf("updated endpoint fields mismatch: got=%#v want=%#v", endpoints, expectedEndpoints)
	}
	if got, err := db.Fallbacks.ListWhitelistUsers(ctx, oneID); err != nil || !reflect.DeepEqual(got, oneWhitelist) {
		t.Fatalf("endpoint one whitelist=%#v err=%v; want exact %#v", got, err, oneWhitelist)
	}
	if got, err := db.Fallbacks.ListWhitelistUsers(ctx, twoID); err != nil || !reflect.DeepEqual(got, twoWhitelist) {
		t.Fatalf("endpoint two whitelist=%#v err=%v; want exact %#v", got, err, twoWhitelist)
	}
	if got, err := db.Fallbacks.ListWhitelistUsers(ctx, removeID); err != nil || len(got) != 0 {
		t.Fatalf("removed endpoint whitelist=%#v err=%v; want empty", got, err)
	}
}

func TestSettingsFallbackStrategyChangesPreserveEndpointsAndWhitelistsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	actor := settingsActor("site_settings.update.any")
	if err := svc.UpdateGroup(ctx, actor, "fallback", map[string]any{
		"fallback_strategy":       "serial",
		"fallback_probe_interval": 600,
		"fallbacks": []any{map[string]any{
			"priority":         1,
			"session_url":      "https://strategy.example/session",
			"account_url":      "https://strategy.example/account",
			"services_url":     "https://strategy.example/services",
			"cache_ttl":        120,
			"skin_domains":     []any{"strategy.example"},
			"enable_profile":   true,
			"enable_hasjoined": false,
			"enable_whitelist": true,
			"note":             "strategy",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	endpointsBefore, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(endpointsBefore) != 1 {
		t.Fatalf("initial strategy endpoints=%#v err=%v", endpointsBefore, err)
	}
	endpointID := endpointsBefore[0]["id"].(int)
	if err := db.Fallbacks.AddWhitelistUser(ctx, "StrategyPlayer", endpointID); err != nil {
		t.Fatal(err)
	}
	whitelistBefore, err := db.Fallbacks.ListWhitelistUsers(ctx, endpointID)
	if err != nil || len(whitelistBefore) != 1 {
		t.Fatalf("initial strategy whitelist=%#v err=%v", whitelistBefore, err)
	}

	for _, change := range []struct {
		strategy string
		interval int
	}{
		{strategy: "parallel", interval: 1200},
		{strategy: "serial", interval: 1800},
	} {
		if err := svc.UpdateGroup(ctx, actor, "fallback", map[string]any{
			"fallback_strategy":       change.strategy,
			"fallback_probe_interval": change.interval,
		}); err != nil {
			t.Fatal(err)
		}
		group, err := svc.GetGroup(ctx, "fallback")
		if err != nil {
			t.Fatal(err)
		}
		if group["fallback_strategy"] != change.strategy || group["fallback_probe_interval"] != change.interval {
			t.Fatalf("strategy group=%#v; want strategy=%q interval=%d", group, change.strategy, change.interval)
		}
		if !reflect.DeepEqual(group["fallbacks"], endpointsBefore) {
			t.Fatalf("strategy-only save changed endpoints: got=%#v want=%#v", group["fallbacks"], endpointsBefore)
		}
		whitelistAfter, err := db.Fallbacks.ListWhitelistUsers(ctx, endpointID)
		if err != nil || !reflect.DeepEqual(whitelistAfter, whitelistBefore) {
			t.Fatalf("strategy-only save changed whitelist: got=%#v err=%v want=%#v", whitelistAfter, err, whitelistBefore)
		}
	}

	err = svc.UpdateGroup(ctx, actor, "fallback", map[string]any{"fallback_strategy": "random"})
	if !settingsHTTPError(err, http.StatusBadRequest, "fallback_strategy.validate.invalid") {
		t.Fatalf("invalid strategy error=%#v; want exact HTTP 400", err)
	}
	group, err := svc.GetGroup(ctx, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if group["fallback_strategy"] != "serial" || group["fallback_probe_interval"] != 1800 || !reflect.DeepEqual(group["fallbacks"], endpointsBefore) {
		t.Fatalf("invalid strategy changed persisted state: %#v", group)
	}
}

func settingsActor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "settings-test",
		UserID:      "settings-test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func settingsHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Error() == detail
}
