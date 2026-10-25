package integration_test

import (
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountDeletionEndsSessionWithoutAnAdditionalLogout(t *testing.T) {
	db, router, cache := testutil.NewTestAppWithRedisTB(t)
	ctx := t.Context()
	user := testutil.CreateUser(t, db, "deleted-session@test.com", "Password123", "DeletedSession", false)
	login := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": user.Email, "password": "Password123"})
	if login.Code != 200 || parseJSON(t, login)["user_id"] != user.ID {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	refresh := cookieNamed(login, "refresh_token")
	if len(cookies) != 2 || refresh == nil || refresh.Value == "" {
		t.Fatalf("login cookies=%#v; want access and refresh cookies", cookies)
	}
	me := doJSON(t, router, "GET", "/v2/users/me", nil, cookies...)
	if me.Code != 200 || parseJSON(t, me)["id"] != user.ID {
		t.Fatalf("initial session status=%d body=%s", me.Code, me.Body.String())
	}
	ygg := model.Token{AccessToken: "deleted-" + user.ID, UserID: user.ID}
	if err := cache.SetYggToken(ctx, ygg, time.Hour); err != nil {
		t.Fatal(err)
	}
	deleted := doJSON(t, router, "DELETE", "/v2/users/me", nil, cookies...)
	if deleted.Code != 204 || deleted.Body.String() != "" {
		t.Fatalf("delete status=%d body=%q; want empty 204", deleted.Code, deleted.Body.String())
	}
	cleared := deleted.Result().Cookies()
	if len(cleared) != 2 || cleared[0].Name != "access_token" || cleared[1].Name != "refresh_token" || cleared[0].Value != "" || cleared[1].Value != "" || cleared[0].MaxAge != -1 || cleared[1].MaxAge != -1 {
		t.Fatalf("delete cookies=%#v; want exactly two expired session cookies", cleared)
	}
	if stored, err := db.Users.GetByID(ctx, user.ID); err != nil || stored != nil {
		t.Fatalf("deleted user=%#v err=%v", stored, err)
	}
	if token, err := db.Tokens.GetRefresh(ctx, util.HashRefreshToken(refresh.Value)); err != nil || token != nil {
		t.Fatalf("delete retained refresh token=%#v err=%v", token, err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete retained auth cache: %v", err)
	}
	if _, err := cache.GetYggToken(ctx, ygg.AccessToken); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete retained Ygg token: %v", err)
	}
	me = doJSON(t, router, "GET", "/v2/users/me", nil, cookies...)
	if me.Code != 401 || me.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("old access cookie status=%d body=%q; want exact authentication denial", me.Code, me.Body.String())
	}
	rotated := doJSON(t, router, "POST", "/v2/auth/session/refresh", nil, refresh)
	if rotated.Code != 401 || rotated.Body.String() != "{\"error\":{\"object\":\"refresh_token\",\"operation\":\"verify\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("old refresh cookie status=%d body=%q; want exact refresh denial", rotated.Code, rotated.Body.String())
	}
}
