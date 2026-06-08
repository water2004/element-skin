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
