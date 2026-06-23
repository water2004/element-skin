package microsoft_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/microsoft"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestMicrosoftRoutesAuthURLAndCallbackValidationExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	states := redisstore.NewMemoryStore()
	h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}, func(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
		return next
	}, states)

	req := httptest.NewRequest(http.MethodGet, "/microsoft/auth-url", nil)
	rec := httptest.NewRecorder()
	h.AuthURL(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "login.live.com") || !strings.Contains(rec.Body.String(), `"state":"`) ||
		states.Len() != 1 {
		t.Fatalf("auth url response mismatch: status=%d body=%q stateLen=%d", rec.Code, rec.Body.String(), states.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?error="+url.QueryEscape("access_denied"), nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Authorization failed: access_denied") {
		t.Fatalf("callback error response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?code=only-code", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Missing code or state parameter"`) {
		t.Fatalf("callback missing state response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?code=code&state=missing", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Invalid or expired state parameter"`) {
		t.Fatalf("callback missing state token response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := microsoft.SeedStateForTest(states, "oauth-state", map[string]any{"kind": microsoft.TestStateKindOAuth, "user_id": "user-id"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?code=code&state=oauth-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://skin.example/root/dashboard/roles?error=auth_failed" {
		t.Fatalf("callback without complete microsoft config should redirect to auth failure: status=%d location=%q body=%q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/microsoft/get-profile", strings.NewReader(`{"ms_token":"missing"}`))
	rec = httptest.NewRecorder()
	h.GetProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid or expired token") {
		t.Fatalf("missing profile token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMicrosoftRoutesSettingsFailuresAndDefaultRedirectConsumeStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = ""
	states := redisstore.NewMemoryStore()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("settings cache unavailable")
	h := microsoft.New(cfg, db, settings.Settings{DB: db, Redis: cache}, func(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
		return next
	}, states)

	req := httptest.NewRequest(http.MethodGet, "/microsoft/auth-url", nil)
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
	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?code=code&state=settings-failure-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" || states.Len() != 0 {
		t.Fatalf("callback settings failure should consume state: status=%d body=%q states=%d", rec.Code, rec.Body.String(), states.Len())
	}

	healthyCache := redisstore.NewMemoryStore()
	h = microsoft.New(cfg, db, settings.Settings{DB: db, Redis: healthyCache}, func(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
		return next
	}, states)
	if err := microsoft.SeedStateForTest(states, "default-site-state", map[string]any{
		"kind": microsoft.TestStateKindOAuth, "user_id": "user-id",
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/microsoft/callback?code=code&state=default-site-state", nil)
	rec = httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/dashboard/roles?error=auth_failed" {
		t.Fatalf("empty site URL fallback mismatch: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}
