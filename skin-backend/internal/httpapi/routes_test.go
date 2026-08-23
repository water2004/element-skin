package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestRoutesRegistersPublicAndYggdrasilEntrypointsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
		exact  bool
	}{
		{method: http.MethodGet, path: "/", status: http.StatusOK, want: "implementationName"},
		{method: http.MethodGet, path: "/v2/public/settings", status: http.StatusOK, want: "site_name"},
		{method: http.MethodPost, path: "/authserver/validate", body: `{"accessToken":"missing"}`, status: http.StatusForbidden, want: "{\"error\":\"ForbiddenOperationException\",\"errorMessage\":\"Invalid token.\"}\n", exact: true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		bodyMatches := rec.Body.String() == tc.want
		if !tc.exact {
			bodyMatches = strings.Contains(rec.Body.String(), tc.want)
		}
		if rec.Code != tc.status || !bodyMatches {
			t.Fatalf("%s %s mismatch: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestRoutesDoNotRetainV1CompatibilityPaths(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	for _, path := range []string{
		"/v1/public/settings",
		"/v1/auth/login",
		"/v1/users/me",
		"/v1/admin/users",
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || rec.Body.String() != "404 page not found\n" {
			t.Fatalf("legacy path %s mismatch: status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}
}
