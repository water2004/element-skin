package integration_test

import (
	"context"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"testing"
	"time"
)

func TestSiteLoginMeAndRefresh(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "api_login@test.com", "ApiPassword123", "LoginUser", false)

	login := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	if login.Code != 200 {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	body := parseJSON(t, login)
	if body["user_id"] != user.ID || body["permissions"] == nil {
		t.Fatalf("unexpected login body: %#v", body)
	}
	access := cookieNamed(login, "access_token")
	refresh := cookieNamed(login, "refresh_token")
	if access == nil || refresh == nil {
		t.Fatalf("missing session cookies: %#v", login.Result().Cookies())
	}

	me := doJSON(t, h, "GET", "/v2/users/me", nil, access)
	if me.Code != 200 {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
	meBody := parseJSON(t, me)
	if meBody["id"] != user.ID {
		t.Fatalf("unexpected me body: %#v", meBody)
	}
	if _, ok := meBody["profiles"]; ok {
		t.Fatalf("/v2/users/me should not inline profiles: %#v", meBody)
	}
	if meBody["profile_count"] != float64(0) || meBody["texture_count"] != float64(0) {
		t.Fatalf("/v2/users/me counts should start at zero: %#v", meBody)
	}
	if err := db.Users.Ban(context.Background(), user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	bannedMe := doJSON(t, h, "GET", "/v2/users/me", nil, access)
	if bannedMe.Code != 200 {
		t.Fatalf("banned user should still access site API, got %d body=%s", bannedMe.Code, bannedMe.Body.String())
	}
	if err := db.Users.Unban(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}

	rotated := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, refresh)
	if rotated.Code != 200 {
		t.Fatalf("refresh status=%d body=%s", rotated.Code, rotated.Body.String())
	}
	newRefresh := cookieNamed(rotated, "refresh_token")
	if newRefresh == nil || newRefresh.Value == refresh.Value {
		t.Fatal("refresh token was not rotated")
	}
	replay := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, refresh)
	if replay.Code != 401 {
		t.Fatalf("old refresh should be rejected, got %d", replay.Code)
	}
	missingRefresh := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil)
	if missingRefresh.Code != 401 {
		t.Fatalf("missing refresh should be 401, got %d", missingRefresh.Code)
	}

	noAccessLogin := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	noAccessRefresh := cookieNamed(noAccessLogin, "refresh_token")
	expiredAccess, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	meExpired := doJSON(t, h, "GET", "/v2/users/me", nil, &http.Cookie{Name: "access_token", Value: expiredAccess})
	if meExpired.Code != 401 {
		t.Fatalf("expired access should be rejected, got %d", meExpired.Code)
	}
	refreshWithoutAccess := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, noAccessRefresh)
	if refreshWithoutAccess.Code != 200 {
		t.Fatalf("refresh should work without valid access, got %d body=%s", refreshWithoutAccess.Code, refreshWithoutAccess.Body.String())
	}

	logoutLogin := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	logoutRefresh := cookieNamed(logoutLogin, "refresh_token")
	logout := doJSON(t, h, "POST", "/v2/auth/logout", nil, logoutRefresh)
	if logout.Code != http.StatusNoContent || logout.Body.Len() != 0 {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}
	afterLogout := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, logoutRefresh)
	if afterLogout.Code != 401 {
		t.Fatalf("refresh after logout should be 401, got %d", afterLogout.Code)
	}

	chpwLogin := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "ApiPassword123"})
	chpwAccess := cookieNamed(chpwLogin, "access_token")
	chpwRefresh := cookieNamed(chpwLogin, "refresh_token")
	chpw := doJSON(t, h, "POST", "/v2/users/me/password", map[string]any{"old_password": "ApiPassword123", "new_password": "NewPassword456!"}, chpwAccess)
	if chpw.Code != http.StatusNoContent || chpw.Body.Len() != 0 {
		t.Fatalf("change password status=%d body=%s", chpw.Code, chpw.Body.String())
	}
	afterPasswordChange := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, chpwRefresh)
	if afterPasswordChange.Code != 401 {
		t.Fatalf("refresh after password change should be 401, got %d", afterPasswordChange.Code)
	}

	deletedUser := testutil.CreateUser(t, db, "refresh_deleted@test.com", "Password123", "RefreshDeleted", false)
	deletedLogin := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": deletedUser.Email, "password": "Password123"})
	deletedRefresh := cookieNamed(deletedLogin, "refresh_token")
	if ok, err := db.Users.Delete(context.Background(), deletedUser.ID); err != nil || !ok {
		t.Fatalf("delete refresh test user ok=%v err=%v", ok, err)
	}
	afterDelete := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, deletedRefresh)
	if afterDelete.Code != 401 {
		t.Fatalf("refresh after user deletion should be 401, got %d", afterDelete.Code)
	}
	deletedAccess, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, deletedUser.ID, time.Hour)
	deletedMe := doJSON(t, h, "GET", "/v2/users/me", nil, &http.Cookie{Name: "access_token", Value: deletedAccess})
	if deletedMe.Code != 401 {
		t.Fatalf("access token for deleted user should be rejected, got %d", deletedMe.Code)
	}
}
