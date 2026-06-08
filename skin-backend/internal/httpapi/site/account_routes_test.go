package site_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/testutil"
)

func TestAccountRoutesMeAndAdminSelfDeleteExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, service.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account@test.com", "Password123", "SiteAccount", false)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) || !strings.Contains(rec.Body.String(), `"email":"site-account@test.com"`) {
		t.Fatalf("me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	adminUser := testutil.CreateUser(t, db, "site-admin-delete@test.com", "Password123", "SiteAdminDelete", true)
	req = httptest.NewRequest(http.MethodDelete, "/me", nil)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "管理员不能删除自己的账号") {
		t.Fatalf("admin self delete should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got, err := db.GetUserByID(req.Context(), adminUser.ID); err != nil || got == nil {
		t.Fatalf("admin should still exist after rejected delete: user=%#v err=%v", got, err)
	}
}
