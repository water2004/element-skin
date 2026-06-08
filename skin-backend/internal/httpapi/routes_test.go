package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestRoutesRegistersPublicAndYggdrasilEntrypointsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	router := httpapi.NewRouter(cfg, db, site.Site{DB: db, Cfg: cfg}, yggdrasil.Yggdrasil{DB: db, Cfg: cfg})

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{method: http.MethodGet, path: "/", status: http.StatusOK, want: "implementationName"},
		{method: http.MethodGet, path: "/public/settings", status: http.StatusOK, want: "site_name"},
		{method: http.MethodPost, path: "/authserver/validate", body: `{"accessToken":"missing"}`, status: http.StatusForbidden, want: "Invalid token"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s %s mismatch: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
