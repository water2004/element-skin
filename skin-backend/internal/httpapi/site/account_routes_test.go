package site_test

import (
	"context"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
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
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account@test.com", "Password123", "SiteAccount", false)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) || !strings.Contains(rec.Body.String(), `"email":"site-account@test.com"`) {
		t.Fatalf("me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	adminUser := testutil.CreateUser(t, db, "site-admin-delete@test.com", "Password123", "SiteAdminDelete", true)
	req = httptest.NewRequest(http.MethodDelete, "/me", nil)
	req = withUserActor(req, adminUser.ID)
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
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account-update@test.com", "Password123", "SiteAccountUpdate", false)
	if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID, IsAdmin: false}, time.Minute); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"display_name":"UpdatedAccount","preferred_language":"en_US"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("update me response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || updated == nil || updated.DisplayName != "UpdatedAccount" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("user update should persist exactly: user=%#v err=%v", updated, err)
	}
	if _, err := redis.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("update me should invalidate auth cache, got %v", err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/site-login", strings.NewReader(`{"email":"site-account-update@test.com","password":"Password123"}`))
	loginRec := httptest.NewRecorder()
	h.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login before change password mismatch: status=%d body=%q", loginRec.Code, loginRec.Body.String())
	}
	refresh := cookieValue(t, loginRec.Result().Cookies(), "refresh_token")
	if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID, IsAdmin: false}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "account_change_password_ygg", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/me/password", strings.NewReader(`{"old_password":"Password123","new_password":"NewPassword123"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "密码修改成功") {
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
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-delete-me@test.com", "Password123", "SiteDeleteMe", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_delete_me_profile", "SiteDeleteMeProfile")
	if err := redis.SetAuthUser(context.Background(), redisstore.AuthUser{ID: user.ID, IsAdmin: false}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "delete_me_ygg", UserID: user.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/me", nil)
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
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
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account-conflict@test.com", "Password123", "SiteAccountConflict", false)
	other := testutil.CreateUser(t, db, "site-account-other@test.com", "Password123", "SiteAccountOther", false)

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"email":"site-account-other@test.com"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Email already in use"`) {
		t.Fatalf("email conflict update mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email {
		t.Fatalf("email conflict should not mutate user: user=%#v err=%v other=%#v", unchanged, err, other)
	}

	req = httptest.NewRequest(http.MethodPost, "/me/password", strings.NewReader(`{"old_password":"WrongPassword","new_password":"NewPassword123"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangePassword(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"旧密码错误"`) {
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
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
		t.Fatalf("me without principal mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/me", nil)
	rec = httptest.NewRecorder()
	h.DeleteMe(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
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
			req := httptest.NewRequest(http.MethodPost, "/me", strings.NewReader(`{`))
			req = withUserActor(req, "malformed-user")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
				t.Fatalf("malformed payload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	if count, err := db.Users.Count(t.Context()); err != nil || count != 0 {
		t.Fatalf("rejected account requests must not mutate users: count=%d err=%v", count, err)
	}
}
