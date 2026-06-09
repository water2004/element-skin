package site_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestSessionRoutesLoginSetsExactCookies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-login@test.com", "Password123", "SiteLogin", false)
	if err := db.Settings.Set(t.Context(), "jwt_expire_days", 2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/site-login", strings.NewReader(`{"email":"site-login@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"user_id":"`+user.ID+`"`) || !strings.Contains(rec.Body.String(), `"is_admin":false`) {
		t.Fatalf("login body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != "access_token" || cookies[1].Name != "refresh_token" || !cookies[0].HttpOnly || !cookies[1].HttpOnly ||
		cookies[0].Path != "/" || cookies[1].Path != "/" || cookies[0].MaxAge != cfg.AccessMinutes*60 || cookies[1].MaxAge != 2*24*3600 {
		t.Fatalf("login should set exact http-only session cookies: %#v", cookies)
	}
}

func TestSessionRoutesRefreshRotatesAndLogoutRevokesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	testutil.CreateUser(t, db, "site-refresh@test.com", "Password123", "SiteRefresh", false)

	req := httptest.NewRequest(http.MethodPost, "/site-login", strings.NewReader(`{"email":"site-refresh@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login before refresh status=%d body=%q", rec.Code, rec.Body.String())
	}
	initialRefresh := cookieValue(t, rec.Result().Cookies(), "refresh_token")
	if initialRefresh == "" {
		t.Fatalf("login should issue refresh token cookies: %#v", rec.Result().Cookies())
	}

	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: initialRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_admin":false`) {
		t.Fatalf("refresh token response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	rotatedRefresh := cookieValue(t, rec.Result().Cookies(), "refresh_token")
	if rotatedRefresh == "" || rotatedRefresh == initialRefresh {
		t.Fatalf("refresh should rotate refresh cookie: old=%q cookies=%#v", initialRefresh, rec.Result().Cookies())
	}
	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: initialRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh token should be single-use after rotation: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rotatedRefresh})
	rec = httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("logout response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookieMaxAge(t, cookies, "access_token") != -1 || cookieMaxAge(t, cookies, "refresh_token") != -1 {
		t.Fatalf("logout should clear both session cookies: %#v", cookies)
	}
	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rotatedRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout should revoke the current refresh token: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"detail":"not authenticated"`) {
		t.Fatalf("refresh without cookie mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func cookieValue(t *testing.T, cookies []*http.Cookie, name string) string {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func cookieMaxAge(t *testing.T, cookies []*http.Cookie, name string) int {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c.MaxAge
		}
	}
	t.Fatalf("missing cookie %q in %#v", name, cookies)
	return 0
}
