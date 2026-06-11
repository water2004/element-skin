package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
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
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true, true))
	rec = httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot change your own admin status") {
		t.Fatalf("self admin toggle should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSuperAdminOnlyAdminRoleControls(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	superAdmin := testutil.CreateUser(t, db, "super-role@test.com", "Password123", "SuperRole", true, true)
	plainAdmin := testutil.CreateUser(t, db, "plain-role@test.com", "Password123", "PlainRole", true)
	target := testutil.CreateUser(t, db, "target-role@test.com", "Password123", "TargetRole", false)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/toggle-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), plainAdmin.ID, true))
	rec := httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "super admin required") {
		t.Fatalf("plain admin toggle should require super admin: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/toggle-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_admin":true`) {
		t.Fatalf("super admin toggle response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/transfer-super-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.TransferSuperAdmin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("transfer super admin response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	oldSuper, err := db.Users.GetByID(req.Context(), superAdmin.ID)
	if err != nil || oldSuper == nil || oldSuper.IsSuperAdmin || !oldSuper.IsAdmin {
		t.Fatalf("old super admin should become plain admin: user=%#v err=%v", oldSuper, err)
	}
	newSuper, err := db.Users.GetByID(req.Context(), target.ID)
	if err != nil || newSuper == nil || !newSuper.IsSuperAdmin || !newSuper.IsAdmin {
		t.Fatalf("target should become super admin: user=%#v err=%v", newSuper, err)
	}
}

func TestSuperAdminRoleControlsRejectExactInvalidTargets(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	superAdmin := testutil.CreateUser(t, db, "super-role-errors@test.com", "Password123", "SuperRoleErrors", true, true)
	plainAdmin := testutil.CreateUser(t, db, "plain-role-errors@test.com", "Password123", "PlainRoleErrors", true)
	target := testutil.CreateUser(t, db, "target-role-errors@test.com", "Password123", "TargetRoleErrors", false)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/transfer-super-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), plainAdmin.ID, true))
	rec := httptest.NewRecorder()
	h.TransferSuperAdmin(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"super admin required\"}\n" {
		t.Fatalf("plain admin transfer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+superAdmin.ID+"/transfer-super-admin", nil)
	req.SetPathValue("user_id", superAdmin.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.TransferSuperAdmin(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"target is already current super admin\"}\n" {
		t.Fatalf("self transfer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/missing-user/transfer-super-admin", nil)
	req.SetPathValue("user_id", "missing-user")
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.TransferSuperAdmin(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing transfer target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	currentSuper, err := db.Users.GetByID(req.Context(), superAdmin.ID)
	if err != nil || currentSuper == nil || !currentSuper.IsSuperAdmin {
		t.Fatalf("invalid transfer attempts should keep current super admin: user=%#v err=%v", currentSuper, err)
	}
}

func TestTransferSuperAdminPreservesRolesWhenTargetCacheInvalidationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := &authInvalidateFailRedis{
		Store:  testutil.NewMemoryRedis(),
		failAt: 2,
	}
	h := admin.NewWithRedis(cfg, db, cache, nil)
	superAdmin := testutil.CreateUser(t, db, "transfer-cache-super@test.com", "Password123", "TransferCacheSuper", true, true)
	target := testutil.CreateUser(t, db, "transfer-cache-target@test.com", "Password123", "TransferCacheTarget", false)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/transfer-super-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec := httptest.NewRecorder()

	h.TransferSuperAdmin(rec, req)

	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("transfer cache failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(cache.userIDs) != 2 || cache.userIDs[0] != superAdmin.ID || cache.userIDs[1] != target.ID {
		t.Fatalf("cache invalidations = %#v, want [%q %q]", cache.userIDs, superAdmin.ID, target.ID)
	}
	unchangedSuper, err := db.Users.GetByID(t.Context(), superAdmin.ID)
	if err != nil || unchangedSuper == nil || !unchangedSuper.IsAdmin || !unchangedSuper.IsSuperAdmin {
		t.Fatalf("failed transfer must preserve current super admin: user=%#v err=%v", unchangedSuper, err)
	}
	unchangedTarget, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || unchangedTarget == nil || unchangedTarget.IsAdmin || unchangedTarget.IsSuperAdmin {
		t.Fatalf("failed transfer must preserve target roles: user=%#v err=%v", unchangedTarget, err)
	}
}

func TestTransferSuperAdminInvalidatesCachesAgainAfterCommit(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	baseCache := testutil.NewMemoryRedis()
	superAdmin := testutil.CreateUser(t, db, "transfer-recache-super@test.com", "Password123", "TransferRecacheSuper", true, true)
	target := testutil.CreateUser(t, db, "transfer-recache-target@test.com", "Password123", "TransferRecacheTarget", false)
	cache := &repopulateDuringTransferRedis{
		Store: baseCache,
		oldUsers: map[string]redisstore.AuthUser{
			superAdmin.ID: redisstore.AuthUserFromModel(superAdmin),
			target.ID:     redisstore.AuthUserFromModel(target),
		},
	}
	h := admin.NewWithRedis(cfg, db, cache, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/transfer-super-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec := httptest.NewRecorder()
	h.TransferSuperAdmin(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("transfer response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	wantInvalidations := []string{superAdmin.ID, target.ID, superAdmin.ID, target.ID}
	if len(cache.userIDs) != len(wantInvalidations) {
		t.Fatalf("cache invalidations=%#v, want %#v", cache.userIDs, wantInvalidations)
	}
	for i, userID := range wantInvalidations {
		if cache.userIDs[i] != userID {
			t.Fatalf("cache invalidations=%#v, want %#v", cache.userIDs, wantInvalidations)
		}
	}
	for _, userID := range []string{superAdmin.ID, target.ID} {
		if _, err := baseCache.GetAuthUser(t.Context(), userID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("post-commit cache for %q must be invalidated, got %v", userID, err)
		}
	}
	oldSuper, err := db.Users.GetByID(t.Context(), superAdmin.ID)
	if err != nil || oldSuper == nil || oldSuper.IsSuperAdmin || !oldSuper.IsAdmin {
		t.Fatalf("source role mismatch after transfer: user=%#v err=%v", oldSuper, err)
	}
	newSuper, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || newSuper == nil || !newSuper.IsSuperAdmin || !newSuper.IsAdmin {
		t.Fatalf("target role mismatch after transfer: user=%#v err=%v", newSuper, err)
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

func TestUserProfilesPaginatesEncodedCursorWithoutRepeatingRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-profile-page@test.com", "Password123", "AdminUserProfilePage", true)
	target := testutil.CreateUser(t, db, "target-user-profile-page@test.com", "Password123", "TargetUserProfilePage", false)
	firstProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_a", "ProfilePageA")
	secondProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_b", "ProfilePageB")

	requestPage := func(cursor string) *httptest.ResponseRecorder {
		targetURL := "/admin/users/" + target.ID + "/profiles?limit=1"
		if cursor != "" {
			targetURL += "&cursor=" + cursor
		}
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)
		req.SetPathValue("user_id", target.ID)
		req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
		rec := httptest.NewRecorder()
		h.UserProfiles(rec, req)
		return rec
	}
	decodePage := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var page map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}

	firstRec := requestPage("")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first user profile page status=%d body=%q", firstRec.Code, firstRec.Body.String())
	}
	first := decodePage(firstRec)
	firstItems := first["items"].([]any)
	cursor, _ := first["next_cursor"].(string)
	if len(firstItems) != 1 || firstItems[0].(map[string]any)["id"] != firstProfile.ID || first["has_next"] != true || cursor == "" {
		t.Fatalf("first user profile page mismatch: %#v", first)
	}

	secondRec := requestPage(cursor)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second user profile page status=%d body=%q", secondRec.Code, secondRec.Body.String())
	}
	second := decodePage(secondRec)
	secondItems := second["items"].([]any)
	if len(secondItems) != 1 || secondItems[0].(map[string]any)["id"] != secondProfile.ID ||
		second["has_next"] != false || second["next_cursor"] != "" {
		t.Fatalf("second user profile page mismatch: %#v", second)
	}

	for _, malformed := range []string{
		"not-base64",
		util.EncodeCursor(map[string]any{"unexpected": "value"}),
	} {
		rec := requestPage(malformed)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
			t.Fatalf("malformed user profile cursor status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
}

func TestUserRoutesMutationsInvalidateAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	superAdmin := testutil.CreateUser(t, db, "admin-cache-super@test.com", "Password123", "AdminCacheSuper", true, true)
	adminUser := testutil.CreateUser(t, db, "admin-cache-admin@test.com", "Password123", "AdminCacheAdmin", true)
	target := testutil.CreateUser(t, db, "admin-cache-target@test.com", "Password123", "AdminCacheTarget", false)

	cacheTarget := func(t *testing.T) {
		t.Helper()
		if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: target.ID, IsAdmin: false}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	assertTargetCacheMiss := func(t *testing.T, action string) {
		t.Helper()
		if _, err := redis.GetAuthUser(t.Context(), target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should invalidate target auth cache, got %v", action, err)
		}
	}

	cacheTarget(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/toggle-admin", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec := httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle admin response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "toggle admin")

	cacheTarget(t)
	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`}`))
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "ban user")

	cacheTarget(t)
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "unban user")

	cacheTarget(t)
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_reset_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminCachePassword123"}`))
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset user password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "reset user password")
	if _, err := redis.GetYggToken(t.Context(), "admin_reset_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("admin reset password should revoke target ygg tokens, got %v", err)
	}
}

func TestUserRoutesRejectInvalidBanUnbanAndResetPayloadsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-errors@test.com", "Password123", "AdminUserErrors", true)
	target := testutil.CreateUser(t, db, "target-user-errors@test.com", "Password123", "TargetUserErrors", false)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{`))
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec := httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("ban bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":1}`))
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"banned_until is required\"}\n" {
		t.Fatalf("ban expired timestamp mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("invalid ban should not change user state: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/missing-user/unban", nil)
	req.SetPathValue("user_id", "missing-user")
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("unban missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`"}`))
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"user_id and new_password required\"}\n" {
		t.Fatalf("reset missing password mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"missing-user","new_password":"AdminNewPassword123"}`))
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("reset missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUnbanReturnsNotFoundWhenUserIsDeletedAfterAuthorizationCheck(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-unban-delete-race@test.com", "Password123", "AdminUnbanRace", true)
	target := testutil.CreateUser(t, db, "target-unban-delete-race@test.com", "Password123", "TargetUnbanRace", false)

	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	var one, lockHolderPID int
	if err := tx.QueryRow(t.Context(), `SELECT 1, pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
		req.SetPathValue("user_id", target.ID)
		req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
		rec := httptest.NewRecorder()
		h.UnbanUser(rec, req)
		result <- rec
	}()
	waitForBlockedAdminMutation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(t.Context(), `DELETE FROM users WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec := <-result
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("user deleted before unban should return exact not found: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func waitForBlockedAdminMutation(
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	lockHolderPID int,
	result <-chan *httptest.ResponseRecorder,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case rec := <-result:
			t.Fatalf("admin mutation completed before row-lock release: status=%d body=%q", rec.Code, rec.Body.String())
		default:
		}
		var waiting bool
		if err := db.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("admin mutation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUserRoutesDeleteUserAndInvalidateAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	adminUser := testutil.CreateUser(t, db, "admin-delete@test.com", "Password123", "AdminDelete", true)
	target := testutil.CreateUser(t, db, "target-delete@test.com", "Password123", "TargetDelete", false)
	profile := testutil.CreateProfile(t, db, target.ID, "delete_user_profile", "DeleteUserProfile")
	if err := redis.SetAuthUser(context.Background(), redisstore.AuthUser{ID: target.ID, IsAdmin: false}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_delete_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID, nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if user, err := db.Users.GetByID(req.Context(), target.ID); err != nil || user != nil {
		t.Fatalf("delete user should remove user row: user=%#v err=%v", user, err)
	}
	if p, err := db.Profiles.GetByID(req.Context(), profile.ID); err != nil || p != nil {
		t.Fatalf("delete user should cascade profile row: profile=%#v err=%v", p, err)
	}
	if _, err := redis.GetAuthUser(context.Background(), target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete user should invalidate auth cache, got %v", err)
	}
	if _, err := redis.GetYggToken(t.Context(), "admin_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete user should revoke existing ygg tokens, got %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID, nil)
	req.SetPathValue("user_id", target.ID)
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"detail":"user not found"`) {
		t.Fatalf("delete missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesProtectSuperAdminFromPlainAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	plainAdmin := testutil.CreateUser(t, db, "plain-protect@test.com", "Password123", "PlainProtect", true)
	superAdmin := testutil.CreateUser(t, db, "super-protect@test.com", "Password123", "SuperProtect", true, true)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+superAdmin.ID, nil)
	req.SetPathValue("user_id", superAdmin.ID)
	req = req.WithContext(shared.WithUser(req.Context(), plainAdmin.ID, true))
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify super admin"`) {
		t.Fatalf("plain admin deleting super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+superAdmin.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`}`))
	req.SetPathValue("user_id", superAdmin.ID)
	req = req.WithContext(shared.WithUser(req.Context(), plainAdmin.ID, true))
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify super admin"`) {
		t.Fatalf("plain admin banning super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+superAdmin.ID+"/unban", nil)
	req.SetPathValue("user_id", superAdmin.ID)
	req = req.WithContext(shared.WithUser(req.Context(), plainAdmin.ID, true))
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"cannot modify super admin\"}\n" {
		t.Fatalf("plain admin unbanning super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesRejectMissingTargetsAndMalformedResetWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	superAdmin := testutil.CreateUser(t, db, "admin-missing-super@test.com", "Password123", "AdminMissingSuper", true, true)
	target := testutil.CreateUser(t, db, "admin-reset-unchanged@test.com", "Password123", "AdminResetUnchanged", false)

	req := httptest.NewRequest(http.MethodGet, "/admin/users?cursor=not-base64", nil)
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	incompleteCursor := util.EncodeCursor(map[string]any{"unexpected": "value"})
	req = httptest.NewRequest(http.MethodGet, "/admin/users?cursor="+incompleteCursor, nil)
	rec = httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list incomplete cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users/missing-user", nil)
	req.SetPathValue("user_id", "missing-user")
	rec = httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing user detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/missing-user/toggle-admin", nil)
	req.SetPathValue("user_id", "missing-user")
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.ToggleUserAdmin(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing admin toggle target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{`))
	req = req.WithContext(shared.WithUser(req.Context(), superAdmin.ID, true, true))
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("malformed reset payload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	unchanged, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("rejected reset must preserve password: user=%#v err=%v", unchanged, err)
	}
}

func TestAdminResetPasswordPreservesCredentialsAndRefreshWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	baseCache := testutil.NewMemoryRedis()
	cache := &deleteYggFailRedis{Store: baseCache}
	h := admin.NewWithRedis(cfg, db, cache, nil)
	adminUser := testutil.CreateUser(t, db, "admin-reset-ygg-fail@test.com", "Password123", "AdminResetYggFail", true)
	target := testutil.CreateUser(t, db, "target-reset-ygg-fail@test.com", "Password123", "TargetResetYggFail", false)
	const refreshHash = "admin_reset_ygg_fail_refresh"
	if err := db.Tokens.AddRefresh(t.Context(), refreshHash, target.ID, time.Now().Add(time.Hour).UnixMilli(), time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
	req = req.WithContext(shared.WithUser(req.Context(), adminUser.ID, true))
	rec := httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("admin reset ygg failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("AdminNewPassword123", unchanged.Password) {
		t.Fatalf("failed admin reset must preserve old password: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(t.Context(), refreshHash); err != nil || refresh == nil || refresh["user_id"] != target.ID {
		t.Fatalf("failed admin reset must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("admin reset should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

type deleteYggFailRedis struct {
	redisstore.Store
	deleteCalls int
}

func (r *deleteYggFailRedis) DeleteYggTokensByUser(context.Context, string) error {
	r.deleteCalls++
	return errors.New("ygg token revocation failed")
}

type authInvalidateFailRedis struct {
	redisstore.Store
	failAt  int
	userIDs []string
}

type repopulateDuringTransferRedis struct {
	redisstore.Store
	oldUsers map[string]redisstore.AuthUser
	userIDs  []string
}

func (r *repopulateDuringTransferRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if err := r.Store.InvalidateAuthUser(ctx, userID); err != nil {
		return err
	}
	if len(r.userIDs) == 2 {
		for _, oldUser := range r.oldUsers {
			if err := r.Store.SetAuthUser(ctx, oldUser, time.Minute); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *authInvalidateFailRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if len(r.userIDs) == r.failAt {
		return errors.New("auth cache invalidation failed")
	}
	return r.Store.InvalidateAuthUser(ctx, userID)
}

func strconvI64(v int64) string {
	return strconv.FormatInt(v, 10)
}
