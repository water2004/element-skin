package publicsite_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	publicsitesvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestPublicSettingsUsesCacheAndFallbackPrimaryEndpointExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	if err := db.Settings.Set(ctx, "site_name", "Cached Site"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "allow_register", false); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", true); err != nil {
		t.Fatal(err)
	}
	if err := db.EmailPolicies.Replace(ctx, model.EmailSuffixPolicy{
		Mode:      model.EmailSuffixModeAllowlist,
		Allowlist: []string{"@example.com", "@qq.com"},
		Denylist:  []string{"@blocked.test"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: "https://session.example", AccountURL: "https://account.example",
		ServicesURL: "https://services.example", CacheTTL: 60, Note: "primary",
	}}); err != nil {
		t.Fatal(err)
	}
	svc := publicsitesvc.Service{
		DB:       db,
		Redis:    redis,
		Settings: settingssvc.Settings{DB: db, Redis: redis},
		SiteURL:  "https://config.example/root/",
		APIURL:   "https://api.example/v2/",
		CacheTTL: time.Minute,
	}

	first, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	status := first["mojang_status_urls"].(map[string]any)
	emailPolicy := first["email_suffix_policy"].(map[string]any)
	suffixes := emailPolicy["suffixes"].([]string)
	if first["site_name"] != "Cached Site" || first["allow_register"] != false ||
		first["require_invite"] != true ||
		emailPolicy["mode"] != model.EmailSuffixModeAllowlist || len(suffixes) != 2 || suffixes[0] != "@example.com" || suffixes[1] != "@qq.com" ||
		first["site_url"] != "https://config.example/root" || first["api_url"] != "https://api.example/v2" ||
		status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" {
		t.Fatalf("public settings mismatch: %#v", first)
	}

	if err := db.Settings.Set(ctx, "site_name", "Changed Site"); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if cached["site_name"] != "Cached Site" {
		t.Fatalf("cached public settings should retain first value: %#v", cached)
	}
	if err := redis.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if err := redis.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	refreshed, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if refreshed["site_name"] != "Changed Site" {
		t.Fatalf("invalidated public settings should reload database value: %#v", refreshed)
	}
}

func TestPublicSettingsRefreshesLegacyCacheWithoutRequireInviteExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	if err := db.Settings.Set(ctx, "site_name", "Fresh Site"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "allow_register", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", true); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetPublicSettings(ctx, map[string]any{
		"site_name":      "Legacy Cached Site",
		"allow_register": false,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	svc := publicsitesvc.Service{
		DB:       db,
		Redis:    redis,
		Settings: settingssvc.Settings{DB: db, Redis: redis},
		SiteURL:  "https://site.example",
		APIURL:   "https://api.example",
		CacheTTL: time.Minute,
	}

	got, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if got["site_name"] != "Fresh Site" || got["allow_register"] != true || got["require_invite"] != true {
		t.Fatalf("legacy public settings cache should be refreshed from database: %#v", got)
	}
	cached, err := redis.GetPublicSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cached["site_name"] != "Fresh Site" || cached["allow_register"] != true || cached["require_invite"] != true {
		t.Fatalf("refreshed public settings should be written back to cache: %#v", cached)
	}
}

func TestHomepageMediaUsesCacheAndEnabledOnlyExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	now := database.NowMS()
	enabled := testPublicMedia("public-enabled", true, 0, now)
	disabled := testPublicMedia("public-disabled", false, 1, now)
	if err := db.HomepageMedia.Create(ctx, enabled); err != nil {
		t.Fatal(err)
	}
	if err := db.HomepageMedia.Create(ctx, disabled); err != nil {
		t.Fatal(err)
	}
	svc := publicsitesvc.Service{DB: db, Redis: redis, CacheTTL: time.Minute}

	first, err := svc.HomepageMedia(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != enabled.ID || first[0].Enabled != true {
		t.Fatalf("public homepage media should include only enabled items: %#v", first)
	}

	later := testPublicMedia("public-later", true, 2, now+1)
	if err := db.HomepageMedia.Create(ctx, later); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.HomepageMedia(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 1 || cached[0].ID != enabled.ID {
		t.Fatalf("cached homepage media should not include later row before invalidation: %#v", cached)
	}
	if err := redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		t.Fatal(err)
	}
	refreshed, err := svc.HomepageMedia(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed) != 2 || refreshed[0].ID != enabled.ID || refreshed[1].ID != later.ID {
		t.Fatalf("invalidated homepage media should reload enabled rows in order: %#v", refreshed)
	}
}

func TestFallbackStatusBuildsEndpointHistoryAndLatestExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{
		{Priority: 1, SessionURL: "https://s1.example", AccountURL: "https://a1.example", ServicesURL: "https://v2.example", CacheTTL: 60, Note: "one"},
		{Priority: 2, SessionURL: "https://s2.example", AccountURL: "https://a2.example", ServicesURL: "https://v2.example", CacheTTL: 60, Note: "two"},
	}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(endpoints) != 2 {
		t.Fatalf("fallback endpoints mismatch: endpoints=%#v err=%v", endpoints, err)
	}
	firstID := endpoints[0]["id"].(int)
	secondID := endpoints[1]["id"].(int)
	now := time.Now().Truncate(time.Second)
	if err := redis.AppendProbeSamples(ctx, []redisstore.ProbeSample{
		{EndpointID: firstID, CheckedAt: now.Add(-2 * time.Minute).UnixMilli(), Note: "older", Session: "up", Account: "down", Services: "up"},
		{EndpointID: firstID, CheckedAt: now.Add(-1 * time.Minute).UnixMilli(), Note: "newer", Session: "down", Account: "up", Services: "up"},
	}, redisstore.ProbeHistoryRetention); err != nil {
		t.Fatal(err)
	}

	status, err := (publicsitesvc.Service{DB: db, Redis: redis}).FallbackStatus(ctx, permission.GuestActor(), now)
	if err != nil {
		t.Fatal(err)
	}
	var decoded fallbackStatusForTest
	decodeFallbackStatusForTest(t, status, &decoded)
	if decoded.GeneratedAt != now.UnixMilli() || decoded.RetentionMS != redisstore.ProbeHistoryRetention.Milliseconds() || len(decoded.Endpoints) != 2 {
		t.Fatalf("fallback status envelope mismatch: %#v", status)
	}
	if decoded.Endpoints[0].ID != firstID || decoded.Endpoints[0].Priority != 1 || decoded.Endpoints[0].Note != "one" ||
		decoded.Endpoints[0].SessionURL != "https://s1.example" || decoded.Endpoints[0].AccountURL != "https://a1.example" || decoded.Endpoints[0].ServicesURL != "https://v2.example" {
		t.Fatalf("first endpoint metadata mismatch: %#v", decoded.Endpoints[0])
	}
	if len(decoded.Endpoints[0].History) != 2 || decoded.Endpoints[0].History[0].CheckedAt != now.Add(-2*time.Minute).UnixMilli() ||
		decoded.Endpoints[0].History[1].CheckedAt != now.Add(-1*time.Minute).UnixMilli() {
		t.Fatalf("first endpoint history mismatch: %#v", decoded.Endpoints[0].History)
	}
	if decoded.Endpoints[0].Latest == nil || decoded.Endpoints[0].Latest.Session != "down" || decoded.Endpoints[0].Latest.Account != "up" || decoded.Endpoints[0].Latest.Services != "up" {
		t.Fatalf("first endpoint latest mismatch: %#v", decoded.Endpoints[0].Latest)
	}
	if decoded.Endpoints[1].ID != secondID || decoded.Endpoints[1].Latest != nil || len(decoded.Endpoints[1].History) != 0 {
		t.Fatalf("second endpoint should have no history: %#v", decoded.Endpoints[1])
	}
}

func TestPublicSitePropagatesRedisErrorsExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	boom := errors.New("redis unavailable")
	memory, ok := redis.(*redisstore.MemoryStore)
	if !ok {
		t.Skip("exact Redis error injection requires memory store")
	}
	memory.Err = boom
	svc := publicsitesvc.Service{DB: db, Redis: redis, Settings: settingssvc.Settings{DB: db, Redis: redis}, CacheTTL: time.Minute}

	if got, err := svc.PublicSettings(ctx, permission.GuestActor()); !errors.Is(err, boom) || got != nil {
		t.Fatalf("PublicSettings redis error mismatch: settings=%#v err=%v", got, err)
	}
	if got, err := svc.HomepageMedia(ctx, permission.GuestActor()); !errors.Is(err, boom) || got != nil {
		t.Fatalf("HomepageMedia redis error mismatch: media=%#v err=%v", got, err)
	}
	if got, err := svc.FallbackStatus(ctx, permission.GuestActor(), time.Unix(1, 0)); !errors.Is(err, boom) || got != nil {
		t.Fatalf("FallbackStatus redis error mismatch: status=%#v err=%v", got, err)
	}
}

func TestPublicSiteServiceRequiresPublicPermissionExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	svc := publicsitesvc.Service{DB: db, Redis: redis, Settings: settingssvc.Settings{DB: db, Redis: redis}}
	if got, err := svc.PublicSettings(t.Context(), permission.Actor{}); got != nil || !publicSiteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("PublicSettings result=%#v err=%#v", got, err)
	}
	if got, err := svc.HomepageMedia(t.Context(), permission.Actor{}); got != nil || !publicSiteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("HomepageMedia result=%#v err=%#v", got, err)
	}
	if got, err := svc.FallbackStatus(t.Context(), permission.Actor{}, time.Unix(1, 0)); got != nil || !publicSiteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("FallbackStatus result=%#v err=%#v", got, err)
	}
}

func publicSiteHTTPError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}

type fallbackStatusForTest struct {
	Endpoints   []fallbackStatusEntryForTest `json:"endpoints"`
	RetentionMS int64                        `json:"retention_ms"`
	GeneratedAt int64                        `json:"generated_at"`
}

type fallbackStatusEntryForTest struct {
	ID          int                         `json:"id"`
	Priority    int                         `json:"priority"`
	Note        string                      `json:"note"`
	SessionURL  string                      `json:"session_url"`
	AccountURL  string                      `json:"account_url"`
	ServicesURL string                      `json:"services_url"`
	Latest      *fallbackStatusTickForTest  `json:"latest"`
	History     []fallbackStatusTickForTest `json:"history"`
}

type fallbackStatusTickForTest struct {
	CheckedAt int64  `json:"checked_at"`
	Session   string `json:"session"`
	Account   string `json:"account"`
	Services  string `json:"services"`
}

func decodeFallbackStatusForTest(t *testing.T, value map[string]any, out *fallbackStatusForTest) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}

func testPublicMedia(id string, enabled bool, order int, now int64) model.HomepageMedia {
	return model.HomepageMedia{
		ID: id, Type: "image", Title: id, StoragePath: id + ".png",
		OverlayOpacityLight: 0.45, OverlayOpacityDark: 0.45, YawSpeedDPS: 4,
		SortOrder: order, Enabled: enabled, DurationMS: 6000, CreatedAt: now, UpdatedAt: now,
	}
}
