package integration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestRedisBackedPublicSettingsAndCarouselHTTP(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	if err := db.Settings.Set(ctx, "site_name", "Redis Public"); err != nil {
		t.Fatal(err)
	}

	first := doJSON(t, h, "GET", "/public/settings", nil)
	if first.Code != 200 || parseJSON(t, first)["site_name"] != "Redis Public" {
		t.Fatalf("first public settings mismatch: %d %s", first.Code, first.Body.String())
	}
	if err := db.Settings.Set(ctx, "site_name", "DB Changed Without Invalidation"); err != nil {
		t.Fatal(err)
	}
	cached := doJSON(t, h, "GET", "/public/settings", nil)
	if cached.Code != 200 || parseJSON(t, cached)["site_name"] != "Redis Public" {
		t.Fatalf("public settings should be served from redis cache: %d %s", cached.Code, cached.Body.String())
	}
	if err := redis.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	refreshed := doJSON(t, h, "GET", "/public/settings", nil)
	if refreshed.Code != 200 || parseJSON(t, refreshed)["site_name"] != "DB Changed Without Invalidation" {
		t.Fatalf("public settings should refresh after invalidation: %d %s", refreshed.Code, refreshed.Body.String())
	}

	if err := redis.SetPublicCarousel(ctx, []string{"cached.png"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	carousel := doJSON(t, h, "GET", "/public/carousel", nil)
	if carousel.Code != 200 || !strings.Contains(carousel.Body.String(), "cached.png") {
		t.Fatalf("public carousel should be served from redis cache: %d %s", carousel.Code, carousel.Body.String())
	}
}

func TestAdminSettingsInvalidatePublicCacheAndApplySecurityImmediately(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "redis-settings-admin@test.com", "Password123", "RedisSettingsAdmin", true)
	token, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminCookie := &http.Cookie{Name: "access_token", Value: token}

	if err := db.Settings.Set(ctx, "site_name", "Cached Site"); err != nil {
		t.Fatal(err)
	}
	first := doJSON(t, h, "GET", "/public/settings", nil)
	if first.Code != 200 || parseJSON(t, first)["site_name"] != "Cached Site" {
		t.Fatalf("prime public settings cache failed: %d %s", first.Code, first.Body.String())
	}
	saveSite := doJSON(t, h, "POST", "/admin/settings/site", map[string]any{"site_name": "Admin Saved Site"}, adminCookie)
	if saveSite.Code != 200 {
		t.Fatalf("save site settings status=%d body=%s", saveSite.Code, saveSite.Body.String())
	}
	afterSite := doJSON(t, h, "GET", "/public/settings", nil)
	if afterSite.Code != 200 || parseJSON(t, afterSite)["site_name"] != "Admin Saved Site" {
		t.Fatalf("public settings cache should be invalidated by admin site save: %d %s", afterSite.Code, afterSite.Body.String())
	}

	if err := redis.SetPublicSettings(ctx, map[string]any{
		"site_name": "Stale Fallback Cache",
		"mojang_status_urls": map[string]any{
			"session":  "stale",
			"account":  "stale",
			"services": "stale",
		},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	saveFallback := doJSON(t, h, "POST", "/admin/settings/fallback", map[string]any{
		"fallbacks": []map[string]any{{
			"priority":     1,
			"session_url":  "https://session.cache",
			"account_url":  "https://account.cache",
			"services_url": "https://services.cache",
			"cache_ttl":    60,
		}},
	}, adminCookie)
	if saveFallback.Code != 200 {
		t.Fatalf("save fallback settings status=%d body=%s", saveFallback.Code, saveFallback.Body.String())
	}
	afterFallback := parseJSON(t, doJSON(t, h, "GET", "/public/settings", nil))
	status := afterFallback["mojang_status_urls"].(map[string]any)
	if status["session"] != "https://session.cache" || status["account"] != "https://account.cache" || status["services"] != "https://services.cache" {
		t.Fatalf("fallback save should invalidate public settings cache: %#v", status)
	}

	saveSecurity := doJSON(t, h, "POST", "/admin/settings/security", map[string]any{
		"rate_limit_enabled":       true,
		"rate_limit_auth_attempts": 1,
		"rate_limit_auth_window":   1,
	}, adminCookie)
	if saveSecurity.Code != 200 {
		t.Fatalf("save security settings status=%d body=%s", saveSecurity.Code, saveSecurity.Body.String())
	}
	firstLogin := doJSONFromIP(t, h, "POST", "/site-login", map[string]any{"email": "missing@test.com", "password": "bad"}, "203.0.113.77:10000")
	if firstLogin.Code != 401 {
		t.Fatalf("first login should reach auth path, got %d %s", firstLogin.Code, firstLogin.Body.String())
	}
	limited := doJSONFromIP(t, h, "POST", "/site-login", map[string]any{"email": "missing@test.com", "password": "bad"}, "203.0.113.77:10000")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("security settings should apply immediately to rate limiter, got %d %s", limited.Code, limited.Body.String())
	}

	saveAuth := doJSON(t, h, "POST", "/admin/settings/auth", map[string]any{"jwt_expire_days": 2}, adminCookie)
	if saveAuth.Code != 200 {
		t.Fatalf("save auth settings status=%d body=%s", saveAuth.Code, saveAuth.Body.String())
	}
	login := doJSON(t, h, "POST", "/site-login", map[string]any{"email": admin.Email, "password": "Password123"})
	if login.Code != 200 {
		t.Fatalf("login after auth settings status=%d body=%s", login.Code, login.Body.String())
	}
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil || refresh.MaxAge != 2*24*3600 {
		t.Fatalf("auth settings should apply to refresh cookie max age: %#v", refresh)
	}
}

func TestRedisBackedRateLimitAndVerificationHTTP(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	if err := db.Settings.Set(ctx, "rate_limit_enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "rate_limit_auth_attempts", 2); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "rate_limit_auth_window", 1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		resp := doJSONFromIP(t, h, "POST", "/site-login", map[string]any{"email": "missing@test.com", "password": "bad"}, "198.51.100.10:10000")
		if resp.Code != 401 {
			t.Fatalf("login attempt %d should reach auth path, got %d %s", i+1, resp.Code, resp.Body.String())
		}
	}
	limited := doJSONFromIP(t, h, "POST", "/site-login", map[string]any{"email": "missing@test.com", "password": "bad"}, "198.51.100.10:10000")
	if limited.Code != http.StatusTooManyRequests || limited.Result().Header.Get("Retry-After") == "" {
		t.Fatalf("third login should be rate limited by redis, got %d %s", limited.Code, limited.Body.String())
	}

	if err := db.Settings.Set(ctx, "rate_limit_enabled", false); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_enabled", true); err != nil {
		t.Fatal(err)
	}
	send := doJSON(t, h, "POST", "/send-verification-code", map[string]any{"email": "redis-register@test.com", "type": "register"})
	if send.Code != 200 {
		t.Fatalf("send verification status=%d body=%s", send.Code, send.Body.String())
	}
	code, err := redis.GetVerificationCode(ctx, "redis-register@test.com", "register")
	if err != nil || len(code) != 8 {
		t.Fatalf("verification code should be stored in redis, code=%q err=%v", code, err)
	}
	if _, _, ok, err := db.Verifications.GetCode(ctx, "redis-register@test.com", "register"); err != nil || ok {
		t.Fatalf("verification code must not be persisted in database: ok=%v err=%v", ok, err)
	}
	register := doJSON(t, h, "POST", "/register", map[string]any{
		"email": "redis-register@test.com", "password": "Password123!", "username": "RedisRegister", "code": strings.ToLower(code),
	})
	if register.Code != 200 {
		t.Fatalf("register with redis code status=%d body=%s", register.Code, register.Body.String())
	}
	if _, err := redis.GetVerificationCode(ctx, "redis-register@test.com", "register"); err == nil {
		t.Fatal("register should consume redis verification code")
	}
}

func TestRedisBackedAuthCacheAndInvalidationHTTP(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "redis-auth-admin@test.com", "Password123", "RedisAuthAdmin", true)
	token, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "access_token", Value: token}
	users := doJSON(t, h, "GET", "/admin/users", nil, cookie)
	if users.Code != 200 {
		t.Fatalf("admin users status=%d body=%s", users.Code, users.Body.String())
	}
	cached, err := redis.GetAuthUser(ctx, admin.ID)
	if err != nil || !cached.IsAdmin {
		t.Fatalf("auth user should be cached in redis: %#v err=%v", cached, err)
	}
	if _, err := db.Users.ToggleAdmin(ctx, admin.ID); err != nil {
		t.Fatal(err)
	}
	stillCached := doJSON(t, h, "GET", "/admin/users", nil, cookie)
	if stillCached.Code != 200 {
		t.Fatalf("cached admin should remain allowed until invalidation, got %d", stillCached.Code)
	}
	if err := redis.InvalidateAuthUser(ctx, admin.ID); err != nil {
		t.Fatal(err)
	}
	demoted := doJSON(t, h, "GET", "/admin/users", nil, cookie)
	if demoted.Code != 403 {
		t.Fatalf("demoted admin should be forbidden after redis invalidation, got %d %s", demoted.Code, demoted.Body.String())
	}

}
