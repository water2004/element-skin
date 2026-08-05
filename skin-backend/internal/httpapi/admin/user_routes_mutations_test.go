package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

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
		if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: target.ID}, time.Minute); err != nil {
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
	req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withProtectedActor(req, superAdmin.ID)
	rec := httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("grant admin role response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "grant admin role")

	cacheTarget(t)
	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`,"reason":"mutation route ban"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"banned_until\":"+strconvI64(banUntil)+"}\n" {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "ban user")

	cacheTarget(t)
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "unban user")

	cacheTarget(t)
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_reset_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminCachePassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
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

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("ban bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":1}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"banned_until is required\"}\n" {
		t.Fatalf("ban expired timestamp mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("invalid ban should not change user state: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"reason is required\"}\n" {
		t.Fatalf("ban missing reason mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("missing reason ban should not change user state: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/missing-user/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("unban missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"user_id and new_password required\"}\n" {
		t.Fatalf("reset missing password mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"missing-user","new_password":"AdminNewPassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
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
		req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("user_id", target.ID)
		req = withAdminActor(req, adminUser.ID)
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

func TestUserRoutesDeleteUserAndInvalidateAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	adminUser := testutil.CreateUser(t, db, "admin-delete@test.com", "Password123", "AdminDelete", true)
	target := testutil.CreateUser(t, db, "target-delete@test.com", "Password123", "TargetDelete", false)
	profile := testutil.CreateProfile(t, db, target.ID, "delete_user_profile", "DeleteUserProfile")
	if err := redis.SetAuthUser(context.Background(), redisstore.AuthUser{ID: target.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_delete_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
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

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"detail":"user not found"`) {
		t.Fatalf("delete missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserMutationRoutesPersistChangesBeforeAuthInvalidationFailureExactly(t *testing.T) {
	t.Run("grant role", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-grant-invalidate@test.com", "Password123", "AdminGrantInvalidate", true)
		target := testutil.CreateUser(t, db, "target-grant-invalidate@test.com", "Password123", "TargetGrantInvalidate", false)

		req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		req.SetPathValue("role_id", permission.RoleAdmin)
		rec := httptest.NewRecorder()
		h.GrantUserRole(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("grant role cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if hasRole, err := db.Permissions.UserHasRole(t.Context(), target.ID, permission.RoleAdmin); err != nil || !hasRole {
			t.Fatalf("grant role should persist before cache failure: has=%v err=%v", hasRole, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("grant role cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})

	t.Run("revoke role", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-revoke-invalidate@test.com", "Password123", "AdminRevokeInvalidate", true)
		target := testutil.CreateUser(t, db, "target-revoke-invalidate@test.com", "Password123", "TargetRevokeInvalidate", false)
		if err := db.Permissions.GrantRole(t.Context(), target.ID, permission.RoleAdmin, permissiondb.SubjectIDForUser(adminUser.ID)); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		req.SetPathValue("role_id", permission.RoleAdmin)
		rec := httptest.NewRecorder()
		h.RevokeUserRole(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("revoke role cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if hasRole, err := db.Permissions.UserHasRole(t.Context(), target.ID, permission.RoleAdmin); err != nil || hasRole {
			t.Fatalf("revoke role should persist before cache failure: has=%v err=%v", hasRole, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("revoke role cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})

	t.Run("ban user", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-ban-invalidate@test.com", "Password123", "AdminBanInvalidate", true)
		target := testutil.CreateUser(t, db, "target-ban-invalidate@test.com", "Password123", "TargetBanInvalidate", false)
		bannedUntil := time.Now().Add(2 * time.Hour).UnixMilli()

		req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(bannedUntil)+`,"reason":"cache failure ban"}`))
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec := httptest.NewRecorder()
		h.BanUser(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("ban user cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		updated, err := db.Users.GetByID(t.Context(), target.ID)
		if err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
			t.Fatalf("ban should persist exact banned_until before cache failure: user=%#v err=%v", updated, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("ban user cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})

	t.Run("unban user", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-unban-invalidate@test.com", "Password123", "AdminUnbanInvalidate", true)
		target := testutil.CreateUser(t, db, "target-unban-invalidate@test.com", "Password123", "TargetUnbanInvalidate", false)
		if err := db.Users.Ban(t.Context(), target.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec := httptest.NewRecorder()
		h.UnbanUser(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("unban user cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		updated, err := db.Users.GetByID(t.Context(), target.ID)
		if err != nil || updated == nil || updated.BannedUntil != nil {
			t.Fatalf("unban should clear banned_until before cache failure: user=%#v err=%v", updated, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("unban user cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})

	t.Run("delete user", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-delete-invalidate@test.com", "Password123", "AdminDeleteInvalidate", true)
		target := testutil.CreateUser(t, db, "target-delete-invalidate@test.com", "Password123", "TargetDeleteInvalidate", false)

		req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID, nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec := httptest.NewRecorder()
		h.DeleteUser(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("delete user cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if user, err := db.Users.GetByID(t.Context(), target.ID); err != nil || user != nil {
			t.Fatalf("delete should remove user before cache failure: user=%#v err=%v", user, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("delete user cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})

	t.Run("reset password", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cache := &authInvalidateFailRedis{Store: testutil.NewMemoryRedis(), failAt: 1}
		h := admin.NewWithRedis(cfg, db, cache, nil)
		adminUser := testutil.CreateUser(t, db, "admin-reset-invalidate@test.com", "Password123", "AdminResetInvalidate", true)
		target := testutil.CreateUser(t, db, "target-reset-invalidate@test.com", "Password123", "TargetResetInvalidate", false)
		const refreshHash = "reset_invalidate_refresh"
		if err := db.Tokens.AddRefresh(t.Context(), refreshHash, target.ID, time.Now().Add(time.Hour).UnixMilli(), time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"ChangedPassword123"}`))
		req = withAdminActor(req, adminUser.ID)
		rec := httptest.NewRecorder()
		h.ResetUserPassword(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("reset password cache failure response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		updated, err := db.Users.GetByID(t.Context(), target.ID)
		if err != nil || updated == nil || !util.VerifyPassword("ChangedPassword123", updated.Password) || util.VerifyPassword("Password123", updated.Password) {
			t.Fatalf("reset should change password before cache failure: user=%#v err=%v", updated, err)
		}
		if refresh, err := db.Tokens.GetRefresh(t.Context(), refreshHash); err != nil || refresh != nil {
			t.Fatalf("reset should revoke refresh token before cache failure: refresh=%#v err=%v", refresh, err)
		}
		if len(cache.userIDs) != 1 || cache.userIDs[0] != target.ID {
			t.Fatalf("reset password cache invalidation targets mismatch: %#v", cache.userIDs)
		}
	})
}
