package microsoft_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/microsoft"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestMicrosoftRoutesAuthURLAndCallbackValidationExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	states := redisstore.NewMemoryStore()
	settingsCache := testutil.NewMemoryRedis()
	if err := settingsCache.SetSetting(context.Background(), "microsoft_client_id", "client-id", time.Minute); err != nil {
		t.Fatal(err)
	}
	h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: settingsCache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
		return next
	}, states)

	req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/auth-url", nil)
	req = withUserActor(req, "microsoft-auth-user")
	rec := httptest.NewRecorder()
	h.AuthURL(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "login.live.com") || !strings.Contains(rec.Body.String(), `"state":"`) ||
		states.Len() != 1 {
		t.Fatalf("auth url response mismatch: status=%d body=%q stateLen=%d", rec.Code, rec.Body.String(), states.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/auth-url", nil)
	req = withUserActorWithoutPermission(req, "microsoft-auth-user", "microsoft_import.start.owned")
	rec = httptest.NewRecorder()
	h.AuthURL(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" || states.Len() != 1 {
		t.Fatalf("auth URL permission denial mismatch: status=%d body=%q stateLen=%d", rec.Code, rec.Body.String(), states.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?error="+url.QueryEscape("access_denied"), nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Authorization failed: access_denied") {
		t.Fatalf("callback error response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=only-code", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Missing code or state parameter"`) {
		t.Fatalf("callback missing state response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=missing", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Invalid or expired state parameter"`) {
		t.Fatalf("callback missing state token response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := microsoft.SeedStateForTest(states, "wrong-kind-state", map[string]any{"kind": microsoft.TestStateKindProfile, "user_id": "user-id"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=wrong-kind-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid or expired state parameter\"}\n" || states.Len() != 1 {
		t.Fatalf("callback wrong state kind mismatch: status=%d body=%q stateLen=%d", rec.Code, rec.Body.String(), states.Len())
	}

	if err := microsoft.SeedStateForTest(states, "oauth-state", map[string]any{"kind": microsoft.TestStateKindOAuth, "user_id": "user-id"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=oauth-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://skin.example/root/dashboard/roles?error=auth_failed" {
		t.Fatalf("callback without complete microsoft config should redirect to auth failure: status=%d location=%q body=%q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/microsoft/profile", strings.NewReader(`{"ms_token":"missing"}`))
	req = withUserActor(req, "microsoft-profile-user")
	rec = httptest.NewRecorder()
	h.GetProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid or expired token") {
		t.Fatalf("missing profile token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMicrosoftRoutesSettingsFailuresAndMissingSiteURLConsumeStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = ""
	states := redisstore.NewMemoryStore()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("settings cache unavailable")
	h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
		return next
	}, states)

	req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/auth-url", nil)
	req = withUserActor(req, "microsoft-auth-settings-user")
	rec := httptest.NewRecorder()
	h.AuthURL(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
		t.Fatalf("auth URL settings failure mismatch: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
	}

	if err := microsoft.SeedStateForTest(states, "settings-failure-state", map[string]any{
		"kind": microsoft.TestStateKindOAuth, "user_id": "user-id",
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=settings-failure-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
		t.Fatalf("callback settings failure should consume state: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
	}

	healthyCache := redisstore.NewMemoryStore()
	h = microsoft.New(cfg, db, settings.Settings{DB: db, Redis: healthyCache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
		return next
	}, states)
	if err := microsoft.SeedStateForTest(states, "default-site-state", map[string]any{
		"kind": microsoft.TestStateKindOAuth, "user_id": "user-id",
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=default-site-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"site URL is not configured\"}\n" || rec.Header().Get("Location") != "" {
		t.Fatalf("empty site URL mismatch: status=%d location=%q body=%q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
}

func TestMicrosoftRoutesReturnExactErrorsForLaterDependencyFailures(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.TestConfig()
	cfg.APIURL = "https://api.example/root"

	t.Run("auth URL redirect URI settings failure keeps state empty", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		states := redisstore.NewMemoryStore()
		cache := redisstore.NewMemoryStore()
		if err := cache.SetSetting(ctx, "microsoft_client_id", "client-id", time.Minute); err != nil {
			t.Fatalf("seed client id cache: %v", err)
		}
		db.Close()

		h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
			return next
		}, states)
		req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/auth-url", nil)
		req = withUserActor(req, "microsoft-auth-redirect-failure-user")
		rec := httptest.NewRecorder()
		h.AuthURL(rec, req)

		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
			t.Fatalf("auth URL redirect settings failure mismatch: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
		}
	})

	t.Run("auth URL state store failure keeps state empty", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		states := redisstore.NewMemoryStore()
		states.Err = errors.New("state store unavailable")
		cache := testutil.NewMemoryRedis()
		if err := cache.SetSetting(ctx, "microsoft_client_id", "client-id", time.Minute); err != nil {
			t.Fatal(err)
		}
		h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
			return next
		}, states)
		req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/auth-url", nil)
		req = withUserActor(req, "microsoft-auth-state-failure-user")
		rec := httptest.NewRecorder()
		h.AuthURL(rec, req)

		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
			t.Fatalf("auth URL state failure mismatch: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
		}
	})

	t.Run("callback client secret settings failure consumes state", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		states := redisstore.NewMemoryStore()
		cache := redisstore.NewMemoryStore()
		if err := cache.SetSetting(ctx, "microsoft_client_id", "client-id", time.Minute); err != nil {
			t.Fatalf("seed client id cache: %v", err)
		}
		if err := microsoft.SeedStateForTest(states, "client-secret-failure-state", map[string]any{
			"kind": microsoft.TestStateKindOAuth, "user_id": "user-id",
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
		db.Close()

		h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
			return next
		}, states)
		req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=client-secret-failure-state", nil)
		rec := httptest.NewRecorder()
		h.Callback(rec, req)

		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
			t.Fatalf("callback client secret settings failure mismatch: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
		}
	})

	t.Run("callback redirect URI settings failure consumes state", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		states := redisstore.NewMemoryStore()
		cache := redisstore.NewMemoryStore()
		if err := cache.SetSetting(ctx, "microsoft_client_id", "client-id", time.Minute); err != nil {
			t.Fatalf("seed client id cache: %v", err)
		}
		if err := cache.SetSetting(ctx, "microsoft_client_secret", "client-secret", time.Minute); err != nil {
			t.Fatalf("seed client secret cache: %v", err)
		}
		if err := microsoft.SeedStateForTest(states, "redirect-failure-state", map[string]any{
			"kind": microsoft.TestStateKindOAuth, "user_id": "user-id",
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
		db.Close()

		h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, _ ...permission.Definition) http.HandlerFunc {
			return next
		}, states)
		req := httptest.NewRequest(http.MethodGet, "/v2/imports/microsoft/callback?code=code&state=redirect-failure-state", nil)
		rec := httptest.NewRecorder()
		h.Callback(rec, req)

		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
			t.Fatalf("callback redirect settings failure mismatch: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
		}
	})
}
