package site_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSessionRoutesLoginSetsExactCookies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-login@test.com", "Password123", "SiteLogin", false)
	if err := db.Settings.Set(t.Context(), "jwt_expire_days", 2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"site-login@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"user_id":"`+user.ID+`"`) || !strings.Contains(rec.Body.String(), `"permissions":[`) {
		t.Fatalf("login body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != "access_token" || cookies[1].Name != "refresh_token" || !cookies[0].HttpOnly || !cookies[1].HttpOnly ||
		cookies[0].Path != "/" || cookies[1].Path != "/" || cookies[0].MaxAge != cfg.AccessMinutes*60 || cookies[1].MaxAge != 2*24*3600 {
		t.Fatalf("login should set exact http-only session cookies: %#v", cookies)
	}
}

func TestSessionRoutesAuthRateLimitIsScopedByForwardedClientIP(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	testutil.CreateUser(t, db, "site-rate-limit@test.com", "Password123", "SiteRateLimit", false)
	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(t.Context(), "rate_limit_auth_window", "1"); err != nil {
		t.Fatal(err)
	}

	first := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"site-rate-limit@test.com","password":"Password123"}`))
	first.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.1")
	rec := httptest.NewRecorder()
	h.Login(rec, first)
	if rec.Code != http.StatusOK {
		t.Fatalf("first login from forwarded IP should pass: status=%d body=%q", rec.Code, rec.Body.String())
	}

	second := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"site-rate-limit@test.com","password":"Password123"}`))
	second.Header.Set("X-Forwarded-For", "203.0.113.9")
	rec = httptest.NewRecorder()
	h.Login(rec, second)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" ||
		rec.Body.String() != "{\"detail\":\"Too many requests, please try again later\"}\n" {
		t.Fatalf("second login from same forwarded IP should be rate-limited: status=%d retry=%q body=%q", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}

	otherIP := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"site-rate-limit@test.com","password":"Password123"}`))
	otherIP.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec = httptest.NewRecorder()
	h.Login(rec, otherIP)
	if rec.Code != http.StatusOK {
		t.Fatalf("login from a different forwarded IP should not inherit the first IP limit: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSessionRoutesRefreshRotatesAndLogoutRevokesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	testutil.CreateUser(t, db, "site-refresh@test.com", "Password123", "SiteRefresh", false)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"site-refresh@test.com","password":"Password123"}`))
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"permissions":[`) {
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

func TestSessionRoutesLogoutReportsRevokeFailureWithoutClearingCookies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "logout-failure@test.com", "Password123", "LogoutFailure", false)

	const rawRefresh = "logout-failure-refresh"
	refreshHash := util.HashRefreshToken(rawRefresh)
	now := time.Now().UnixMilli()
	if err := db.Tokens.AddRefresh(t.Context(), refreshHash, user.ID, now+int64(time.Hour/time.Millisecond), now); err != nil {
		t.Fatalf("add refresh: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rawRefresh})
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("logout status = %d, want %d; body = %q", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if got, want := rec.Body.String(), "{\"detail\":\"Internal server error\"}\n"; got != want {
		t.Fatalf("logout body = %q, want %q", got, want)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("logout failure cookies = %#v, want none", cookies)
	}
	stored, err := db.Tokens.GetRefresh(t.Context(), refreshHash)
	if err != nil {
		t.Fatalf("get refresh after failed logout: %v", err)
	}
	if stored == nil || stored["user_id"] != user.ID {
		t.Fatalf("refresh after failed logout = %#v, want token for %q", stored, user.ID)
	}
}

func TestSessionRoutesRegisterCreatesFirstAdminAndProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	if err := db.Settings.Set(t.Context(), "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", strings.NewReader(`{"email":"new-user@test.com","password":"Password123","username":"New User"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`) {
		t.Fatalf("register response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	id := jsonStringField(t, rec.Body.String(), "id")
	user, err := db.Users.GetByID(req.Context(), id)
	if err != nil || user == nil || user.Email != "new-user@test.com" || user.DisplayName != "New User" {
		t.Fatalf("first registered user should be super admin exactly: user=%#v err=%v", user, err)
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), id, "super_admin"); err != nil || !hasRole {
		t.Fatalf("first registered user role = %v, %v; want super_admin", hasRole, err)
	}
	profiles, err := db.Profiles.GetByUser(req.Context(), id, 10)
	if err != nil || len(profiles) != 1 || profiles[0].Name != "new_user" || profiles[0].ID != util.OfflineUUIDNoDash("new_user") {
		t.Fatalf("register should create exact offline profile: profiles=%#v err=%v", profiles, err)
	}
}

func TestSessionRoutesVerificationAndResetPasswordExactFlow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "reset-flow@test.com", "Password123", "ResetFlow", false)

	req := httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"reset"}`))
	rec := httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Email verification is disabled"`) {
		t.Fatalf("verification disabled response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := db.Settings.Set(t.Context(), "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(t.Context(), "email_verify_ttl", "123"); err != nil {
		t.Fatal(err)
	}
	if err := redis.InvalidateSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "reset_password_ygg", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"reset"}`))
	rec = httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"ttl\":123}\n" {
		t.Fatalf("send reset verification response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	code, err := redis.GetVerificationCode(t.Context(), "reset-flow@test.com", "reset")
	if err != nil || code == "" {
		t.Fatalf("reset verification code should be stored in redis: code=%q err=%v", code, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/auth/password/reset", strings.NewReader(`{"email":"reset-flow@test.com","password":"NewPassword123","code":"`+code+`"}`))
	rec = httptest.NewRecorder()
	h.ResetPassword(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reset password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || updated == nil || !util.VerifyPassword("NewPassword123", updated.Password) {
		t.Fatalf("reset password should update user password: user=%#v err=%v", updated, err)
	}
	if _, err := redis.GetVerificationCode(t.Context(), "reset-flow@test.com", "reset"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset password should delete verification code, got %v", err)
	}
	if _, err := redis.GetYggToken(t.Context(), "reset_password_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset password should revoke existing ygg tokens, got %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"bad"}`))
	rec = httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid verification type"`) {
		t.Fatalf("invalid verification type response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSessionRoutesRejectMalformedAndIncompletePayloadsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)

	tests := []struct {
		name   string
		target string
		body   string
		call   func(http.ResponseWriter, *http.Request)
		want   string
	}{
		{name: "login malformed json", target: "/v1/auth/login", body: `{`, call: h.Login, want: "{\"detail\":\"invalid json\"}\n"},
		{name: "register malformed json", target: "/v1/auth/register", body: `{`, call: h.Register, want: "{\"detail\":\"invalid json\"}\n"},
		{name: "verification malformed json", target: "/verification-code", body: `{`, call: h.SendVerificationCode, want: "{\"detail\":\"invalid json\"}\n"},
		{name: "verification missing email", target: "/verification-code", body: `{"type":"register"}`, call: h.SendVerificationCode, want: "{\"detail\":\"email required\"}\n"},
		{name: "reset malformed json", target: "/v1/auth/password/reset", body: `{`, call: h.ResetPassword, want: "{\"detail\":\"invalid json\"}\n"},
		{name: "reset missing code", target: "/v1/auth/password/reset", body: `{"email":"person@test.com","password":"NewPassword123"}`, call: h.ResetPassword, want: "{\"detail\":\"email, password and code required\"}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != tc.want {
				t.Fatalf("status=%d body=%q, want 400 %q", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}

	if count, err := db.Users.Count(t.Context()); err != nil || count != 0 {
		t.Fatalf("rejected session requests must not create users: count=%d err=%v", count, err)
	}
}

func TestSessionRoutesFailClosedOnRateLimitConfigurationAndStoreErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "rate-limit-failure@test.com", "Password123", "RateLimitFailure", false)

	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "not-an-int"); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"rate-limit-failure@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"user_id":"`+user.ID+`"`) {
		t.Fatalf("invalid rate-limit integer should fall back to default: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := db.Settings.Set(t.Context(), "rate_limit_auth_attempts", "5"); err != nil {
		t.Fatal(err)
	}
	if err := cache.InvalidateSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	failedUser := testutil.CreateUser(t, db, "rate-limit-store-failure@test.com", "Password123", "RateLimitStoreFailure", false)
	failingCache := &rateLimitFailRedis{Store: cache}
	h = site.NewWithRedis(cfg, db, failingCache, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	req = httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"rate-limit-store-failure@test.com","password":"Password123"}`))
	rec = httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("rate-limit store failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	var refreshCount int
	err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM site_refresh_tokens WHERE user_id=$1`, failedUser.ID).Scan(&refreshCount)
	if err != nil || refreshCount != 0 {
		t.Fatalf("failed-closed login must not issue sessions: count=%d err=%v", refreshCount, err)
	}
}

type rateLimitFailRedis struct {
	redisstore.Store
}

func (r *rateLimitFailRedis) HitRateLimit(context.Context, string, int, time.Duration) (redisstore.RateLimitResult, error) {
	return redisstore.RateLimitResult{}, errors.New("redis unavailable")
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
