package httpapi_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestRouterServeHTTPAddsAuthlibHeaderAndAuthRoutes(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.APIURL = "https://api.example/root"
	user := testutil.CreateUser(t, db, "auth-user@test.com", "Password123", "AuthUser", false)
	admin := testutil.CreateUser(t, db, "auth-admin@test.com", "Password123", "AuthAdmin", true)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("X-Authlib-Injector-API-Location") != "https://api.example/root" {
		t.Fatalf("missing authlib API header: %q", rec.Header().Get("X-Authlib-Injector-API-Location"))
	}

	userToken, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := util.CreateAccessToken(cfg.JWTSecret, admin.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: userToken})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"id":"`+user.ID+`"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"permissions":[`)) {
		t.Fatalf("/v2/users/me auth response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: userToken})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte("permission denied")) {
		t.Fatalf("non-admin should be forbidden: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users?limit=1", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: adminToken})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"page_size":1`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"has_next":true`)) {
		t.Fatalf("admin should list users exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/users/me", nil))
	if rec.Code != http.StatusUnauthorized || !bytes.Contains(rec.Body.Bytes(), []byte("not authenticated")) {
		t.Fatalf("missing cookie should be unauthorized: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRouterRegistersRepresentativeRouteGroups(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "route-user@test.com", "Password123", "RouteUser", false)
	admin := testutil.CreateUser(t, db, "route-admin@test.com", "Password123", "RouteAdmin", true)
	userToken, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := util.CreateAccessToken(cfg.JWTSecret, admin.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	cases := []struct {
		name       string
		method     string
		path       string
		token      string
		body       string
		wantStatus int
		wantBody   string
	}{
		{name: "metadata", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "implementationName"},
		{name: "public settings", method: http.MethodGet, path: "/v2/public/settings", wantStatus: http.StatusOK, wantBody: "site_name"},
		{name: "me route", method: http.MethodGet, path: "/v2/users/me", token: userToken, wantStatus: http.StatusOK, wantBody: user.ID},
		{name: "admin settings", method: http.MethodGet, path: "/v2/admin/settings/site", token: adminToken, wantStatus: http.StatusOK, wantBody: "site_name"},
		{name: "ygg validate invalid", method: http.MethodPost, path: "/authserver/validate", body: `{"accessToken":"missing"}`, wantStatus: http.StatusForbidden, wantBody: "Invalid token"},
		{name: "remote ygg", method: http.MethodPost, path: "/v2/imports/remote-ygg/profiles/preview", token: userToken, body: `{}`, wantStatus: http.StatusBadRequest, wantBody: "api_url, username and password are required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				req.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus || !bytes.Contains(rec.Body.Bytes(), []byte(tc.wantBody)) {
				t.Fatalf("%s route mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouterAppliesConfiguredCORSExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.APIURL = "https://api.example/root"
	cfg.CORSOrigins = []string{"https://app.example", "http://localhost:5173"}
	cfg.CORSCredentials = true
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/v2/public/settings", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example" ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "true" ||
		rec.Header().Values("Vary")[0] != "Origin" ||
		rec.Header().Get("X-Authlib-Injector-API-Location") != "https://api.example/root" {
		t.Fatalf("cors simple response mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodOptions, "/v2/users/me", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent ||
		rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, PATCH, DELETE, OPTIONS" ||
		rec.Header().Get("Access-Control-Allow-Headers") != "content-type, authorization" ||
		rec.Header().Get("Access-Control-Max-Age") != "600" ||
		rec.Body.String() != "" {
		t.Fatalf("cors preflight response mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodOptions, "/v2/users/me", nil)
	req.Header.Set("Origin", "https://blocked.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("blocked preflight mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestRouterRejectsCredentialedWildcardCORSExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CORSOrigins = []string{"*"}
	cfg.CORSCredentials = true
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/v2/public/settings", nil)
	req.Header.Set("Origin", "https://any.example")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		rec.Header().Get("Access-Control-Allow-Origin") != "" ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("credentialed wildcard cors mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodOptions, "/v2/users/me", nil)
	req.Header.Set("Origin", "https://any.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "Forbidden\n" || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("credentialed wildcard preflight mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}

	cfg.CORSCredentials = false
	router = httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	req = httptest.NewRequest(http.MethodGet, "/v2/public/settings", nil)
	req.Header.Set("Origin", "https://any.example")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		rec.Header().Get("Access-Control-Allow-Origin") != "*" ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("non-credentialed wildcard cors mismatch: status=%d headers=%#v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}
}
