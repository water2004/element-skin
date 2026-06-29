package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestSettingsRoutesSaveSiteSettingsPersistsValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/settings/site", strings.NewReader(`{"site_name":"Route Site"}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	got, err := db.Settings.Get(req.Context(), "site_name", "")
	if err != nil || got != "Route Site" {
		t.Fatalf("site setting should persist exactly: got=%q err=%v", got, err)
	}
}

func TestSettingsRoutesGetAndSaveSiteSettingsInvalidateCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(testutil.TestConfig(), db, redis, nil)
	ctx := context.Background()

	if err := db.Settings.Set(ctx, "site_name", "Cached Site"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/settings/site", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.GetSiteSettings(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name":"Cached Site"`) ||
		!strings.Contains(rec.Body.String(), `"allow_register":true`) ||
		!strings.Contains(rec.Body.String(), `"profile_uuid_mode":"random"`) {
		t.Fatalf("get site settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if cached, err := redis.GetSetting(ctx, "site_name"); err != nil || cached != "Cached Site" {
		t.Fatalf("get site settings should populate setting cache: cached=%q err=%v", cached, err)
	}
	if err := redis.SetPublicSettings(ctx, map[string]any{"site_name": "stale"}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/settings/site", strings.NewReader(`{"site_name":"Fresh Site","profile_uuid_mode":"offline","unknown_key":"ignored"}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save site settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := redis.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("save site settings should invalidate setting cache, got %v", err)
	}
	if _, err := redis.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("save site settings should invalidate public settings cache, got %v", err)
	}
	got, err := db.Settings.Get(ctx, "site_name", "")
	if err != nil || got != "Fresh Site" {
		t.Fatalf("site_name should persist after save: got=%q err=%v", got, err)
	}
	mode, err := db.Settings.Get(ctx, "profile_uuid_mode", "")
	if err != nil || mode != "offline" {
		t.Fatalf("profile_uuid_mode should persist after save: got=%q err=%v", mode, err)
	}
	ignored, err := db.Settings.Get(ctx, "unknown_key", "")
	if err != nil || ignored != "" {
		t.Fatalf("unknown settings keys should be ignored: got=%q err=%v", ignored, err)
	}
}

func TestSettingsRoutesGetAndSaveNamedGroupExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/settings/security", strings.NewReader(`{"rate_limit_enabled":true,"rate_limit_auth_attempts":9}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "security")
	rec := httptest.NewRecorder()
	h.SaveSettingsGroup(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save security settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/settings/security", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "security")
	rec = httptest.NewRecorder()
	h.GetSettingsGroup(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"rate_limit_enabled":true`) || !strings.Contains(rec.Body.String(), `"rate_limit_auth_attempts":9`) {
		t.Fatalf("get security settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSettingsRoutesNamedGroupsInvalidateOnlyRelevantPublicCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(testutil.TestConfig(), db, redis, nil)

	for _, tc := range []struct {
		group string
		body  string
	}{
		{"fallback", `{"fallback_strategy":"parallel"}`},
		{"email", `{"smtp_sender":"skin@example.com"}`},
		{"easter_eggs", `{"easter_eggs_enabled":["christmas"]}`},
	} {
		t.Run(tc.group, func(t *testing.T) {
			if err := redis.SetPublicSettings(t.Context(), map[string]any{"site_name": "stale"}, time.Minute); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/admin/settings/"+tc.group, strings.NewReader(tc.body))
			req = withAdminActor(req, "admin-test-user")
			req.SetPathValue("group", tc.group)
			rec := httptest.NewRecorder()
			h.SaveSettingsGroup(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
				t.Fatalf("save %s group response mismatch: status=%d body=%q", tc.group, rec.Code, rec.Body.String())
			}
			if _, err := redis.GetPublicSettings(t.Context()); !errors.Is(err, redisstore.ErrCacheMiss) {
				t.Fatalf("save %s group should invalidate public settings cache, got %v", tc.group, err)
			}
		})
	}

	if err := redis.SetPublicSettings(t.Context(), map[string]any{"site_name": "still-fresh"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/settings/security", strings.NewReader(`{"rate_limit_auth_attempts":7}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "security")
	rec := httptest.NewRecorder()
	h.SaveSettingsGroup(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save security group response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := redis.GetPublicSettings(t.Context())
	if err != nil || cached["site_name"] != "still-fresh" {
		t.Fatalf("save security group should not invalidate public settings cache: cached=%#v err=%v", cached, err)
	}
}

func TestSettingsRoutesRejectInvalidGroupAndBadJSONExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/settings/nope", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "nope")
	rec := httptest.NewRecorder()
	h.GetSettingsGroup(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid settings group"`) {
		t.Fatalf("invalid group get mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/settings/site", strings.NewReader(`{"site_name":`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid json"`) {
		t.Fatalf("bad site settings json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/settings/site", strings.NewReader(`{"profile_uuid_mode":"bad"}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid profile_uuid_mode"`) {
		t.Fatalf("invalid profile uuid mode mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSettingsRoutesReturnErrorWhenCacheInvalidationFailsAfterPersist(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := &invalidateFailRedis{Store: testutil.NewMemoryRedis(), failSettings: true}
	h := admin.NewWithRedis(testutil.TestConfig(), db, redis, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/settings/site", strings.NewReader(`{"site_name":"Persisted Despite Cache Failure"}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("site save should expose generic error when settings cache invalidation fails: status=%d body=%q", rec.Code, rec.Body.String())
	}
	got, err := db.Settings.Get(req.Context(), "site_name", "")
	if err != nil || got != "Persisted Despite Cache Failure" {
		t.Fatalf("site setting should persist before cache invalidation failure: got=%q err=%v", got, err)
	}

	redis.failSettings = false
	redis.failPublic = true
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/settings/easter_eggs", strings.NewReader(`{"easter_eggs_enabled":["christmas"]}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "easter_eggs")
	rec = httptest.NewRecorder()
	h.SaveSettingsGroup(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("named public group save should fail when public cache invalidation fails: status=%d body=%q", rec.Code, rec.Body.String())
	}
	raw, err := db.Settings.Get(req.Context(), "easter_eggs_enabled", "")
	if err != nil || raw != `["christmas"]` {
		t.Fatalf("easter egg settings should persist before public cache invalidation failure: got=%q err=%v", raw, err)
	}

	redis.failPublic = false
	if err := redis.SetPublicSettings(req.Context(), map[string]any{"site_name": "still-fresh"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/settings/security", strings.NewReader(`{"rate_limit_auth_attempts":11}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("group", "security")
	rec = httptest.NewRecorder()
	h.SaveSettingsGroup(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("non-public settings group should not invalidate public cache: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := redis.GetPublicSettings(req.Context())
	if err != nil || cached["site_name"] != "still-fresh" {
		t.Fatalf("non-public settings group should keep public cache intact: cached=%#v err=%v", cached, err)
	}
}

type invalidateFailRedis struct {
	redisstore.Store
	failSettings bool
	failPublic   bool
}

func (r *invalidateFailRedis) InvalidateSettings(ctx context.Context) error {
	if r.failSettings {
		return errors.New("settings cache invalidation failed")
	}
	return r.Store.InvalidateSettings(ctx)
}

func (r *invalidateFailRedis) InvalidatePublicSettings(ctx context.Context) error {
	if r.failPublic {
		return errors.New("public cache invalidation failed")
	}
	return r.Store.InvalidatePublicSettings(ctx)
}
