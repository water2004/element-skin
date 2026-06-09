package microsoft_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/microsoft"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestMicrosoftRoutesAuthURLAndCallbackValidationExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	states := util.NewInMemoryStateStore()
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

	microsoft.SeedStateForTest(states, "oauth-state", map[string]any{"kind": microsoft.TestStateKindOAuth, "user_id": "user-id"}, time.Minute)
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
