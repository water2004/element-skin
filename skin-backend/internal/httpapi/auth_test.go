package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRejectsMissingInvalidAndNonAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-direct-user@test.com", "Password123", "AuthDirectUser", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/users/me", nil))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("missing cookie auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "not-a-jwt"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("invalid jwt auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "permission denied") {
		t.Fatalf("non-admin auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthRedisErrorDoesNotFallBackToDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-redis-error@test.com", "Password123", "AuthRedisError", true)
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis auth error should fail without DB fallback, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthUsesRedisCachedSubjectIDButRecomputesPermissions(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-hit@test.com", "Password123", "AuthCacheHit", true)
	cache := testutil.NewMemoryRedis()
	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("cached subject ID should still use DB permissions: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthFailsClosedWhenColdCacheCannotBePopulated(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-write@test.com", "Password123", "AuthCacheWrite", false)
	cache := &authCacheWriteFailStore{Store: redisstore.NewMemoryStore()}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("cold-cache write failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if cache.setCalls != 1 {
		t.Fatalf("auth middleware should attempt one cache population, calls=%d", cache.setCalls)
	}
	if _, err := cache.Store.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed cache population must not leave a partial entry, got %v", err)
	}
}

func TestAuthCachesBanStateWithoutBlockingWebDashboard(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-banned-cold@test.com", "Password123", "AuthBannedCold", false)
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if err := db.Users.Ban(t.Context(), user.ID, bannedUntil); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) {
		t.Fatalf("banned web user should keep dashboard access: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetAuthUser(t.Context(), user.ID)
	if err != nil || cached.BannedUntil == nil || *cached.BannedUntil != bannedUntil || !cached.Banned(time.Now()) {
		t.Fatalf("ban state should be cached exactly: cached=%#v err=%v", cached, err)
	}
}

type authCacheWriteFailStore struct {
	redisstore.Store
	setCalls int
}

func (s *authCacheWriteFailStore) SetAuthUser(context.Context, redisstore.AuthUser, time.Duration) error {
	s.setCalls++
	return errors.New("cache write failed")
}
