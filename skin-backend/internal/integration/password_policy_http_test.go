package integration_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	public := doJSON(t, router, "GET", "/v2/public/settings", nil)
	if public.Code != http.StatusOK {
		t.Fatalf("initial public settings status=%d body=%s", public.Code, public.Body.String())
	}
	if _, exposed := parseJSON(t, public)["enable_strong_password_check"]; exposed {
		t.Fatalf("public settings exposed password policy: %s", public.Body.String())
	}
	publicBefore, err := cache.GetPublicSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sequence := 0
	previousEnabled := false
	for _, setting := range passwordPolicySettings {
		// Populate the cache before saving so the API must invalidate the previous policy.
		previous := doJSON(t, router, "GET", "/v2/admin/settings/security", nil, adminCookies...)
		if previous.Code != http.StatusOK || parseJSON(t, previous)["enable_strong_password_check"] != previousEnabled {
			t.Fatalf("read previous security settings status=%d body=%s; want enabled=%t", previous.Code, previous.Body.String(), previousEnabled)
		}
		saved := doJSON(t, router, "POST", "/v2/admin/settings/security", map[string]any{"enable_strong_password_check": setting.value}, adminCookies...)
		if saved.Code != http.StatusNoContent || saved.Body.String() != "" || len(saved.Result().Cookies()) != 0 {
			t.Fatalf("save %s status=%d body=%q cookies=%#v; want 204 without body or cookies", setting.name, saved.Code, saved.Body.String(), saved.Result().Cookies())
		}
		if stored, err := db.Settings.Get(ctx, "enable_strong_password_check", ""); err != nil || stored != setting.stored {
			t.Fatalf("persisted %s policy=%q err=%v; want %q", setting.name, stored, err, setting.stored)
		}
		if cached, err := cache.GetSetting(ctx, "enable_strong_password_check"); !errors.Is(err, redisstore.ErrCacheMiss) || cached != "" {
			t.Fatalf("save %s retained policy cache=%q err=%v; want empty and ErrCacheMiss", setting.name, cached, err)
		}
		if publicAfter, err := cache.GetPublicSettings(ctx); err != nil || !reflect.DeepEqual(publicAfter, publicBefore) {
			t.Fatalf("save %s changed public cache=%#v err=%v; want %#v", setting.name, publicAfter, err, publicBefore)
		}
		updated := doJSON(t, router, "GET", "/v2/admin/settings/security", nil, adminCookies...)
		if updated.Code != http.StatusOK || parseJSON(t, updated)["enable_strong_password_check"] != setting.enabled {
			t.Fatalf("read saved %s status=%d body=%s; want enabled=%t", setting.name, updated.Code, updated.Body.String(), setting.enabled)
		}
		previousEnabled = setting.enabled
		for _, fixture := range testutil.PasswordPolicyCases(t) {
			if !fixture.EntrypointTest {
				continue
			}
			for _, entry := range passwordEntryPoints {
				t.Run(fmt.Sprintf("%s/%s/%s", setting.name, entry.name, fixture.Name), func(t *testing.T) {
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
					if setting.enabled {
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

func TestRejectedPasswordPolicySettingsRequestsPreserveDatabaseAndCaches(t *testing.T) {
	db, router, cache := testutil.NewTestAppWithRedisTB(t)
	admin := testutil.CreateUser(t, db, "policy-settings-admin@test.com", "Password123", "PolicySettingsAdmin", true)
	user := testutil.CreateUser(t, db, "policy-settings-user@test.com", "Password123", "PolicySettingsUser", false)
	adminLogin := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": admin.Email, "password": "Password123"})
	userLogin := doJSON(t, router, "POST", "/v2/auth/login", map[string]string{"email": user.Email, "password": "Password123"})
	if adminLogin.Code != 200 || parseJSON(t, adminLogin)["user_id"] != admin.ID || userLogin.Code != 200 || parseJSON(t, userLogin)["user_id"] != user.ID {
		t.Fatalf("settings fixture login mismatch: admin=%d %s user=%d %s", adminLogin.Code, adminLogin.Body.String(), userLogin.Code, userLogin.Body.String())
	}
	adminCookies := adminLogin.Result().Cookies()
	initial := doJSON(t, router, "GET", "/v2/admin/settings/security", nil, adminCookies...)
	if initial.Code != 200 || parseJSON(t, initial)["enable_strong_password_check"] != false {
		t.Fatalf("initial policy status=%d body=%s; want disabled", initial.Code, initial.Body.String())
	}
	public := doJSON(t, router, "GET", "/v2/public/settings", nil)
	if public.Code != 200 {
		t.Fatalf("public settings status=%d body=%s", public.Code, public.Body.String())
	}
	if _, exposed := parseJSON(t, public)["enable_strong_password_check"]; exposed {
		t.Fatalf("public settings exposed password policy: %s", public.Body.String())
	}
	publicBefore, err := cache.GetPublicSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range rejectedPasswordSettingUpdates {
		t.Run(fixture.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/admin/settings/security", strings.NewReader(fixture.body))
			request.Header.Set("Content-Type", "application/json")
			cookies := userLogin.Result().Cookies()
			if fixture.admin {
				cookies = adminCookies
			}
			for _, cookie := range cookies {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != fixture.status || response.Body.String() != fixture.errorBody || len(response.Result().Cookies()) != 0 {
				t.Fatalf("rejected settings status=%d body=%q cookies=%#v; want %d %q without cookies", response.Code, response.Body.String(), response.Result().Cookies(), fixture.status, fixture.errorBody)
			}
			if stored, err := db.Settings.Get(t.Context(), "enable_strong_password_check", ""); err != nil || stored != "false" {
				t.Fatalf("rejection changed stored policy=%q err=%v; want false", stored, err)
			}
			if cached, err := cache.GetSetting(t.Context(), "enable_strong_password_check"); err != nil || cached != "false" {
				t.Fatalf("rejection changed policy cache=%q err=%v; want false", cached, err)
			}
			if publicAfter, err := cache.GetPublicSettings(t.Context()); err != nil || !reflect.DeepEqual(publicAfter, publicBefore) {
				t.Fatalf("rejection changed public cache=%#v err=%v; want %#v", publicAfter, err, publicBefore)
			}
			after := doJSON(t, router, "GET", "/v2/admin/settings/security", nil, adminCookies...)
			if after.Code != initial.Code || after.Body.String() != initial.Body.String() {
				t.Fatalf("rejection changed security group status=%d body=%q; want %d %q", after.Code, after.Body.String(), initial.Code, initial.Body.String())
			}
		})
	}
}
