package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/testutil"
)

func TestUserRoutesListAndProtectCurrentUserExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-users@test.com", "Password123", "AdminUsers", true)
	other := testutil.CreateUser(t, db, "listed-users@test.com", "Password123", "ListedUsers", false)

	req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=1&q=Listed", nil)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+other.ID+`"`) ||
		!strings.Contains(rec.Body.String(), `"email":"listed-users@test.com"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("admin user list mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+adminUser.ID, nil)
	req.SetPathValue("user_id", adminUser.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot delete yourself") {
		t.Fatalf("self delete should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+adminUser.ID+"/toggle-admin", nil)
	req.SetPathValue("user_id", adminUser.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot change your own admin status") {
		t.Fatalf("self admin toggle should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAdminAuthWrapperRequiresAdmin(t *testing.T) {
	var requireAdmin bool
	h := admin.New(testutil.TestConfig(), nil, func(next http.HandlerFunc, require bool) http.HandlerFunc {
		requireAdmin = require
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.Background()))
	if rec.Code != http.StatusNoContent || !requireAdmin {
		t.Fatalf("admin Auth should request admin access: status=%d requireAdmin=%v", rec.Code, requireAdmin)
	}
}
