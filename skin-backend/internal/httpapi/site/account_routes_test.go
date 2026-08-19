package site_test

import (
	"context"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountRoutesMeAndAdminSelfDeleteExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-account@test.com", "Password123", "SiteAccount", false)

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) ||
		!strings.Contains(rec.Body.String(), `"email":"site-account@test.com"`) ||
		!strings.Contains(rec.Body.String(), `"protected":false`) {
		t.Fatalf("me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	adminUser := testutil.CreateUser(t, db, "site-admin-delete@test.com", "Password123", "SiteAdminDelete", true, true)
	req = httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
	req = withUserActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"protected_subject\",\"operation\":\"delete\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("protected subject self delete should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got, err := db.Users.GetByID(req.Context(), adminUser.ID); err != nil || got == nil {
		t.Fatalf("admin should still exist after rejected delete: user=%#v err=%v", got, err)
	}
}

func TestAccountRoutesUpdateMeAndChangePasswordExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, nil)
	user := testutil.CreateUser(t, db, "site-account-update@test.com", "Password123", "SiteAccountUpdate", false)
	if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v2/users/me", strings.NewReader(`{"display_name":"UpdatedAccount","preferred_language":"en_US"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("update me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || updated == nil || updated.DisplayName != "UpdatedAccount" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("user update should persist exactly: user=%#v err=%v", updated, err)
	}
	if _, err := redis.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("update me should invalidate auth cache, got %v", err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/v2/auth/login", strings.NewReader(`{"email":"site-account-update@test.com","password":"Password123"}`))
	loginRec := httptest.NewRecorder()
	h.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login before change password mismatch: status=%d body=%q", loginRec.Code, loginRec.Body.String())
	}
	refresh := cookieValue(t, loginRec.Result().Cookies(), "refresh_token")
	if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "account_change_password_ygg", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/password", strings.NewReader(`{"old_password":"Password123","new_password":"NewPassword123"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("change password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := redis.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("change password should invalidate auth cache, got %v", err)
	}
	if got, err := db.Tokens.GetRefresh(t.Context(), util.HashRefreshToken(refresh)); err != nil || got != nil {
		t.Fatalf("change password should revoke existing refresh tokens: refresh=%#v err=%v", got, err)
	}
	if _, err := redis.GetYggToken(t.Context(), "account_change_password_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("change password should revoke existing ygg tokens, got %v", err)
	}
}

func TestAccountRoutesDeleteMeRemovesUserAndInvalidatesCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, nil)
	user := testutil.CreateUser(t, db, "site-delete-me@test.com", "Password123", "SiteDeleteMe", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_delete_me_profile", "SiteDeleteMeProfile")
	if err := redis.SetAuthUser(context.Background(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "delete_me_ygg", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got, err := db.Users.GetByID(req.Context(), user.ID); err != nil || got != nil {
		t.Fatalf("delete me should remove user row: user=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(req.Context(), profile.ID); err != nil || got != nil {
		t.Fatalf("delete me should cascade profile row: profile=%#v err=%v", got, err)
	}
	if _, err := redis.GetAuthUser(context.Background(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete me should invalidate auth cache, got %v", err)
	}
	if _, err := redis.GetYggToken(t.Context(), "delete_me_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete me should revoke existing ygg tokens, got %v", err)
	}
}

func TestAccountRoutesRejectConflictsAndWrongOldPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-account-conflict@test.com", "Password123", "SiteAccountConflict", false)
	other := testutil.CreateUser(t, db, "site-account-other@test.com", "Password123", "SiteAccountOther", false)

	req := httptest.NewRequest(http.MethodPatch, "/v2/users/me", strings.NewReader(`{"email":"site-account-other@test.com"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"email\",\"operation\":\"update\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("direct email update mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email {
		t.Fatalf("email conflict should not mutate user: user=%#v err=%v other=%#v", unchanged, err, other)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/password", strings.NewReader(`{"old_password":"WrongPassword","new_password":"NewPassword123"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"password\",\"operation\":\"verify\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("wrong old password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err = db.Users.GetByID(req.Context(), user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("wrong old password should not change hash: user=%#v err=%v", unchanged, err)
	}
}

func TestAccountRoutesRejectMissingPrincipalAndMalformedPayloadsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("me without principal mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("delete without principal mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "update me", call: h.UpdateMe},
		{name: "change password", call: h.ChangePassword},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v2/users/me", strings.NewReader(`{`))
			req = withUserActor(req, "malformed-user")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
				t.Fatalf("malformed payload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	if count, err := db.Users.Count(t.Context()); err != nil || count != 0 {
		t.Fatalf("rejected account requests must not mutate users: count=%d err=%v", count, err)
	}
}

func TestAccountRoutesRejectMissingPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "site-account-perms@test.com", "Password123", "SiteAccountPerms", false)

	cases := []struct {
		name       string
		permission string
		makeReq    func() *http.Request
		call       func(http.ResponseWriter, *http.Request)
	}{
		{name: "me", permission: "account.read.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
		}, call: h.Me},
		{name: "update me", permission: "account.update.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodPatch, "/v2/users/me", strings.NewReader(`{"display_name":"Blocked"}`))
		}, call: h.UpdateMe},
		{name: "delete me", permission: "account.delete.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
		}, call: h.DeleteMe},
		{name: "change password", permission: "account_password.update.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/v2/users/me/password", strings.NewReader(`{"old_password":"Password123","new_password":"NewPassword123"}`))
		}, call: h.ChangePassword},
		{name: "send email change code", permission: "account.update.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/v2/users/me/email/verification-code", strings.NewReader(`{"email":"blocked-new@test.com"}`))
		}, call: h.SendEmailChangeCode},
		{name: "change email", permission: "account.update.self", makeReq: func() *http.Request {
			return httptest.NewRequest(http.MethodPut, "/v2/users/me/email", strings.NewReader(`{"email":"blocked-new@test.com","code":"BLOCKED1"}`))
		}, call: h.ChangeEmail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withUserActorWithoutPermission(tc.makeReq(), user.ID, tc.permission)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("%s permission mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	unchanged, err := db.Users.GetByID(t.Context(), user.ID)
	if err != nil || unchanged == nil || unchanged.DisplayName != "SiteAccountPerms" || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("permission denials must not mutate account: user=%#v err=%v", unchanged, err)
	}
}

type invalidateAuthUserFailStore struct {
	redisstore.Store
}

func (s invalidateAuthUserFailStore) InvalidateAuthUser(context.Context, string) error {
	return errors.New("invalidate auth user failed")
}

func TestAccountRoutesReturnExactErrorWhenAuthCacheInvalidationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	baseRedis := testutil.NewMemoryRedis()
	redis := invalidateAuthUserFailStore{Store: baseRedis}
	h := site.NewWithRedis(cfg, db, redis, nil)
	user := testutil.CreateUser(t, db, "site-account-invalidate@test.com", "Password123", "SiteAccountInvalidate", false)

	req := httptest.NewRequest(http.MethodPatch, "/v2/users/me", strings.NewReader(`{"display_name":"InvalidateChanged"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("update invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(t.Context(), user.ID)
	if err != nil || updated == nil || updated.DisplayName != "InvalidateChanged" {
		t.Fatalf("update should persist before invalidate failure: user=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/users/me/password", strings.NewReader(`{"old_password":"Password123","new_password":"NewPassword123"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("password invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err = db.Users.GetByID(t.Context(), user.ID)
	if err != nil || updated == nil || !util.VerifyPassword("NewPassword123", updated.Password) {
		t.Fatalf("password should persist before invalidate failure: user=%#v err=%v", updated, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("delete invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	preserved, err := db.Users.GetByID(t.Context(), user.ID)
	if err != nil || preserved == nil || preserved.DisplayName != "InvalidateChanged" || !util.VerifyPassword("NewPassword123", preserved.Password) {
		t.Fatalf("delete invalidate failure must preserve exact user: user=%#v err=%v", preserved, err)
	}
}

func TestDeleteMeErrorPaths(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, nil)
	user := testutil.CreateUser(t, db, "site-delete-me-err@test.com", "Password123", "SiteDeleteMeErr", false)

	// DB error on UserIsProtected: cancelled context causes query failure
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil).WithContext(ctx)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("cancelled context should return 500 exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got, err := db.Users.GetByID(context.Background(), user.ID); err != nil || got == nil {
		t.Fatalf("user should still exist after cancelled-context error: user=%#v err=%v", got, err)
	}

	// ok=false from DeleteUser: pre-delete user so Delete returns false
	if _, err := db.Users.Delete(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/v2/users/me", nil)
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"user\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("pre-deleted user should return 404 exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
