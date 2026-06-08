package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
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

func TestUserRoutesDetailProfilesBanUnbanAndResetPassword(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-actions@test.com", "Password123", "AdminUserActions", true)
	target := testutil.CreateUser(t, db, "target-user-actions@test.com", "Password123", "TargetUserActions", false)
	profile := testutil.CreateProfile(t, db, target.ID, "target_user_profile", "TargetUserProfile")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+target.ID, nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec := httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+target.ID+`"`) || !strings.Contains(rec.Body.String(), `"email":"target-user-actions@test.com"`) {
		t.Fatalf("user detail response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users/"+target.ID+"/profiles", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.UserProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"TargetUserProfile"`) {
		t.Fatalf("user profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`}`))
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"banned_until":`+strconvI64(banUntil)) {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || !banned {
		t.Fatalf("target should be banned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("target should be unbanned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reset password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), target.ID)
	if err != nil || updated == nil || !util.VerifyPassword("AdminNewPassword123", updated.Password) {
		t.Fatalf("reset password should persist new hash: user=%#v err=%v", updated, err)
	}
}

func strconvI64(v int64) string {
	return strconv.FormatInt(v, 10)
}
