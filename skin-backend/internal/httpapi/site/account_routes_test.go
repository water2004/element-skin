package site_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
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

	req := httptest.NewRequest(http.MethodDelete, "/me", nil)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
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
}

func TestAccountRoutesRejectConflictsAndWrongOldPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-account-conflict@test.com", "Password123", "SiteAccountConflict", false)
	other := testutil.CreateUser(t, db, "site-account-other@test.com", "Password123", "SiteAccountOther", false)

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"email":"site-account-other@test.com"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
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
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
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
