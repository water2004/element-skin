package integration_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestPasswordEntryPointsApplySamePolicyAndPreserveRejectedState(t *testing.T) {
	db, router, cache := testutil.NewTestAppWithRedisTB(t)
	ctx := t.Context()
	admin := testutil.CreateUser(t, db, "policy-admin@test.com", "Password123", "PolicyAdmin", true)
	adminLogin := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": admin.Email, "password": "Password123"})
	if adminLogin.Code != 200 || parseJSON(t, adminLogin)["user_id"] != admin.ID {
		t.Fatalf("admin login status=%d body=%s", adminLogin.Code, adminLogin.Body.String())
	}
	adminCookies := adminLogin.Result().Cookies()
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	for _, strong := range []bool{false, true} {
		if err := db.Settings.Set(ctx, "enable_strong_password_check", strong); err != nil {
			t.Fatal(err)
		}
		invalidateSettings(t, cache)
		for _, fixture := range testutil.PasswordPolicyCases(t) {
			if !fixture.EntrypointTest {
				continue
			}
			for _, entry := range passwordEntryPoints {
				t.Run(fmt.Sprintf("strong=%t/%s/%s", strong, entry.name, fixture.Name), func(t *testing.T) {
					sequence++
					email := fmt.Sprintf("policy-%d@test.com", sequence)
					username := fmt.Sprintf("PolicyUser%d", sequence)
					body := map[string]string{entry.passwordField: fixture.Password}
					var target model.User
					var before *model.User
					var refreshBefore map[string]any
					var cached redisstore.AuthUser
					var ygg model.Token
					var cookies []*http.Cookie
					if entry.name != "register" {
						target = testutil.CreateUser(t, db, email, "Password123", username, false)
						var err error
						before, err = db.Users.GetByID(ctx, target.ID)
						if err != nil {
							t.Fatal(err)
						}
						if err := db.Tokens.AddRefresh(ctx, target.ID, target.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
							t.Fatal(err)
						}
						refreshBefore, err = db.Tokens.GetRefresh(ctx, target.ID)
						if err != nil {
							t.Fatal(err)
						}
						cached = redisstore.AuthUser{ID: target.ID}
						ygg = model.Token{AccessToken: target.ID, UserID: target.ID, CreatedAt: database.NowMS()}
						if err := cache.SetYggToken(ctx, ygg, time.Hour); err != nil {
							t.Fatal(err)
						}
					}
					purpose := ""
					switch entry.name {
					case "register":
						body["email"], body["username"], body["code"] = email, username, "POLICY12"
						purpose = "register"
					case "email_reset":
						body["email"], body["code"] = email, "POLICY12"
						purpose = "reset"
					case "self":
						body["old_password"] = "Password123"
						login := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": email, "password": "Password123"})
						if login.Code != 200 || parseJSON(t, login)["user_id"] != target.ID {
							t.Fatalf("self login status=%d body=%s", login.Code, login.Body.String())
						}
						cookies = login.Result().Cookies()
					case "admin":
						body["user_id"] = target.ID
						cookies = adminCookies
					}
					if purpose != "" {
						if err := cache.SetVerificationCode(ctx, email, purpose, "POLICY12", time.Hour); err != nil {
							t.Fatal(err)
						}
					}
					if before != nil {
						if err := cache.SetAuthUser(ctx, cached, time.Hour); err != nil {
							t.Fatal(err)
						}
					}
					expectedErrors := fixture.BasicErrors
					if strong {
						expectedErrors = fixture.StrongErrors
					}
					response := doJSON(t, router, "POST", entry.path, body, cookies...)
					stored, err := db.Users.GetByEmail(ctx, email)
					if err != nil {
						t.Fatal(err)
					}
					if len(expectedErrors) != 0 {
						wantBody := passwordPolicyErrorBody(t, expectedErrors)
						if fixture.Password == "" {
							wantBody = entry.emptyErrorBody
						}
						if response.Code != 400 || response.Body.String() != wantBody || len(response.Result().Cookies()) != 0 {
							t.Fatalf("rejection status=%d body=%q cookies=%#v; want 400 %q without cookies", response.Code, response.Body.String(), response.Result().Cookies(), wantBody)
						}
						if !reflect.DeepEqual(stored, before) {
							t.Fatalf("rejection changed stored user for %s", entry.name)
						}
						if before != nil {
							refreshAfter, err := db.Tokens.GetRefresh(ctx, target.ID)
							if err != nil || !reflect.DeepEqual(refreshBefore, refreshAfter) {
								t.Fatalf("rejection changed refresh token: got=%#v err=%v", refreshAfter, err)
							}
							actualCache, err := cache.GetAuthUser(ctx, target.ID)
							if err != nil || !reflect.DeepEqual(actualCache, cached) {
								t.Fatalf("rejection changed auth cache: got=%#v err=%v", actualCache, err)
							}
							actualYgg, err := cache.GetYggToken(ctx, target.ID)
							if err != nil || !reflect.DeepEqual(actualYgg, ygg) {
								t.Fatalf("rejection changed Ygg token: got=%#v err=%v", actualYgg, err)
							}
						}
						if purpose != "" {
							code, err := cache.GetVerificationCode(ctx, email, purpose)
							if err != nil || code != "POLICY12" {
								t.Fatalf("rejection consumed verification code: got=%q err=%v", code, err)
							}
						}
						return
					}
					if response.Code != entry.successStatus || stored == nil || stored.Email != email || !util.VerifyPassword(fixture.Password, stored.Password) || util.VerifyPassword("Password123", stored.Password) {
						t.Fatalf("success status=%d body=%q; want %d and exact new password", response.Code, response.Body.String(), entry.successStatus)
					}
					if entry.name == "register" {
						if parseJSON(t, response)["id"] != stored.ID {
							t.Fatalf("register response=%s does not identify persisted user", response.Body.String())
						}
					} else {
						if response.Body.String() != "" {
							t.Fatalf("password mutation body=%q; want empty", response.Body.String())
						}
						if token, err := db.Tokens.GetRefresh(ctx, target.ID); err != nil || token != nil {
							t.Fatalf("success retained refresh token=%#v err=%v", token, err)
						}
						if _, err := cache.GetYggToken(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
							t.Fatalf("success retained Ygg token: %v", err)
						}
					}
					if purpose != "" {
						if _, err := cache.GetVerificationCode(ctx, email, purpose); !errors.Is(err, redisstore.ErrCacheMiss) {
							t.Fatalf("success retained verification code: %v", err)
						}
					}
				})
			}
		}
	}
}

func TestRejectedAccountDeletionPreservesSessionAndCookies(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "protected-session@test.com", "Password123", "ProtectedSession", true, true)
	login := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": user.Email, "password": "Password123"})
	if login.Code != 200 || parseJSON(t, login)["user_id"] != user.ID {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	refresh := cookieNamed(login, "refresh_token")
	if len(cookies) != 2 || refresh == nil {
		t.Fatalf("login cookies=%#v; want access and refresh cookies", cookies)
	}
	before, err := db.Tokens.GetRefresh(t.Context(), util.HashRefreshToken(refresh.Value))
	if err != nil {
		t.Fatal(err)
	}
	rejected := doJSON(t, router, "DELETE", "/v2/users/me", nil, cookies...)
	if rejected.Code != 403 || rejected.Body.String() != "{\"error\":{\"object\":\"protected_subject\",\"operation\":\"delete\",\"reason\":\"denied\"}}\n" || len(rejected.Result().Cookies()) != 0 {
		t.Fatalf("delete status=%d body=%q cookies=%#v; want exact denial without cookie changes", rejected.Code, rejected.Body.String(), rejected.Result().Cookies())
	}
	after, err := db.Tokens.GetRefresh(t.Context(), util.HashRefreshToken(refresh.Value))
	if err != nil || before == nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected deletion changed refresh token: before=%#v after=%#v err=%v", before, after, err)
	}
	me := doJSON(t, router, "GET", "/v2/users/me", nil, cookies...)
	if me.Code != 200 || parseJSON(t, me)["id"] != user.ID {
		t.Fatalf("session invalid after rejected deletion: status=%d body=%s", me.Code, me.Body.String())
	}
}
