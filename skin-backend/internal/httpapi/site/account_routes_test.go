package site_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestAccountRoutesMeAndAdminSelfDeleteExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
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
	if got, err := db.Users.GetByID(req.Context(), adminUser.ID); err != nil || got == nil {
		t.Fatalf("admin should still exist after rejected delete: user=%#v err=%v", got, err)
	}
}

func TestAccountRoutesUpdateMeAndChangePasswordExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account-update@test.com", "Password123", "SiteAccountUpdate", false)

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"display_name":"UpdatedAccount","preferred_language":"en_US"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("update me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || updated == nil || updated.DisplayName != "UpdatedAccount" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("user update should persist exactly: user=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/me/password", strings.NewReader(`{"old_password":"Password123","new_password":"NewPassword123"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "密码修改成功") {
		t.Fatalf("change password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
