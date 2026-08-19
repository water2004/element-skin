package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	mailsvc "element-skin/backend/internal/service/mail"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type httpapiTestMailSender struct{}

func (httpapiTestMailSender) SendVerificationCode(context.Context, string, string, string) error {
	return nil
}

var _ mailsvc.Sender = httpapiTestMailSender{}

func TestAuthRejectsMissingInvalidAndNonAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-direct-user@test.com", "Password123", "AuthDirectUser", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/users/me", nil))
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("missing cookie auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "not-a-jwt"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("invalid jwt auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("non-admin auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicAuthUsesGuestAndRejectsInvalidOrDeniedAuthenticatedActorsExactly(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/publickeys/", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Authlib-Injector-API-Location") != cfg.APIURL {
		t.Fatalf("guest public keys response mismatch: status=%d header=%q body=%q", rec.Code, rec.Header().Get("X-Authlib-Injector-API-Location"), rec.Body.String())
	}
	var publicKeys model.YggdrasilPublicKeys
	if err := json.Unmarshal(rec.Body.Bytes(), &publicKeys); err != nil {
		t.Fatal(err)
	}
	publicKeyPEM, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	wantKeys, err := fallbacksvc.NormalizePEMPublicKeys([]string{string(publicKeyPEM)})
	if err != nil {
		t.Fatal(err)
	}
	if len(publicKeys.ProfilePropertyKeys) != 1 || len(publicKeys.PlayerCertificateKeys) != 1 || publicKeys.ProfilePropertyKeys[0] != wantKeys[0] || publicKeys.PlayerCertificateKeys[0] != wantKeys[0] {
		t.Fatalf("guest public keys body mismatch: %#v", publicKeys)
	}
	recWithoutSlash := httptest.NewRecorder()
	router.ServeHTTP(recWithoutSlash, httptest.NewRequest(http.MethodGet, "/api/publickeys", nil))
	if recWithoutSlash.Code != http.StatusOK || recWithoutSlash.Body.String() != rec.Body.String() {
		t.Fatalf("public keys aliases diverged: slash=%q no-slash status=%d body=%q", rec.Body.String(), recWithoutSlash.Code, recWithoutSlash.Body.String())
	}

	for _, path := range publicSiteAuthPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer invalid-public-token")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
			t.Fatalf("invalid public bearer mismatch for %s: status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}

	user := testutil.CreateUser(t, db, "public-denied@test.com", "Password123", "PublicDenied", false)
	if err := db.Permissions.SetSubjectPermissionOverride(t.Context(), user.ID, permission.MustDefinitionByCode("site_public.read.public"), "deny", ""); err != nil {
		t.Fatal(err)
	}
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range publicSiteAuthPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
			t.Fatalf("denied authenticated public actor mismatch for %s: status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("guest root after denied user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	var metadata map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	plural, ok := metadata["signaturePublickeys"].([]any)
	if !ok || len(plural) != 1 || plural[0] != metadata["signaturePublickey"] {
		t.Fatalf("root signature keys mismatch: %#v", metadata)
	}
}

func TestPublicV2ResourcesUseGuestAndRejectInvalidOrDeniedCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "public-v2-denied@test.com", "Password123", "PublicV2Denied", false)
	for _, code := range []string{
		"site_public.read.public",
		"texture.read.public",
		"minecraft_profile.read.public",
		"minecraft_texture_property.read.public",
	} {
		if err := db.Permissions.SetSubjectPermissionOverride(t.Context(), user.ID, permission.MustDefinitionByCode(code), "deny", ""); err != nil {
			t.Fatalf("deny %s: %v", code, err)
		}
	}
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	for _, tc := range publicV2ResourceCases {
		t.Run(tc.name, func(t *testing.T) {
			guestReq := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			guestRec := httptest.NewRecorder()
			router.ServeHTTP(guestRec, guestReq)
			if guestRec.Code != tc.guestStatus || (tc.guestBody != "" && guestRec.Body.String() != tc.guestBody) ||
				(tc.guestContains != "" && !strings.Contains(guestRec.Body.String(), tc.guestContains)) {
				t.Fatalf("guest response mismatch: status=%d body=%q", guestRec.Code, guestRec.Body.String())
			}

			invalidReq := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			invalidReq.Header.Set("Authorization", "Bearer invalid-public-v2-token")
			invalidRec := httptest.NewRecorder()
			router.ServeHTTP(invalidRec, invalidReq)
			if invalidRec.Code != http.StatusUnauthorized || invalidRec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
				t.Fatalf("invalid credential response mismatch: status=%d body=%q", invalidRec.Code, invalidRec.Body.String())
			}

			deniedReq := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			deniedReq.AddCookie(&http.Cookie{Name: "access_token", Value: token})
			deniedRec := httptest.NewRecorder()
			router.ServeHTTP(deniedRec, deniedReq)
			if deniedRec.Code != http.StatusForbidden || deniedRec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("denied actor response mismatch: status=%d body=%q", deniedRec.Code, deniedRec.Body.String())
			}
		})
	}
}

func TestAuthRedisErrorDoesNotFallBackToDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-redis-error@test.com", "Password123", "AuthRedisError", true)
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis auth error should fail without DB fallback, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthUsesRedisCachedSubjectIDButRecomputesPermissions(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-hit@test.com", "Password123", "AuthCacheHit", true)
	cache := testutil.NewMemoryRedis()
	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("cached subject ID should still use DB permissions: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthFailsClosedWhenColdCacheCannotBePopulated(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-write@test.com", "Password123", "AuthCacheWrite", false)
	cache := &authCacheWriteFailStore{Store: redisstore.NewMemoryStore()}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("cold-cache write failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if cache.setCalls != 1 {
		t.Fatalf("auth middleware should attempt one cache population, calls=%d", cache.setCalls)
	}
	if _, err := cache.Store.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed cache population must not leave a partial entry, got %v", err)
	}
}

func TestAuthCachesBanStateWithoutBlockingWebDashboard(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-banned-cold@test.com", "Password123", "AuthBannedCold", false)
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if err := db.Users.Ban(t.Context(), user.ID, bannedUntil); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) {
		t.Fatalf("banned web user should keep dashboard access: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetAuthUser(t.Context(), user.ID)
	if err != nil || cached.BannedUntil == nil || *cached.BannedUntil != bannedUntil || !cached.Banned(time.Now()) {
		t.Fatalf("ban state should be cached exactly: cached=%#v err=%v", cached, err)
	}
}

func TestAuthAcceptsDelegatedBearerAndNarrowsPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-bearer-user@test.com", "Password123", "AuthBearerUser", false)
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	clientID := createActiveOAuthClientForAuthTest(t, db, user.ID, "auth-bearer-client", []int64{
		int64(permission.MustDefinitionByCode("account.read.self").ID),
	})
	grantID := "grant-auth-bearer"
	if err := db.OAuth.CreateGrant(t.Context(), model.OAuthGrant{
		ID:        grantID,
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  clientID,
		Status:    "active",
		CreatedAt: database.NowMS(),
	}, []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)}); err != nil {
		t.Fatal(err)
	}
	rawToken := "delegated-bearer-token"
	now := database.NowMS()
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawToken),
		ClientID:      clientID,
		UserID:        user.ID,
		GrantID:       grantID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delegated bearer me status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body struct {
		ID          string   `json:"id"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bearer me: %v body=%q", err, rec.Body.String())
	}
	if body.ID != user.ID {
		t.Fatalf("delegated bearer user id mismatch: got %q want %q", body.ID, user.ID)
	}
	if !containsAuthPermission(body.Permissions, "account.read.self") || containsAuthPermission(body.Permissions, "account.update.self") {
		t.Fatalf("delegated bearer permissions should be narrowed to token scope: %#v", body.Permissions)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/users/me", strings.NewReader(`{"display_name":"ShouldFail"}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("delegated bearer update denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestEmailChangeAcceptsCookieAndDelegatedBearerThroughSamePermissionPath(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	if err := db.Settings.Set(t.Context(), "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg}, httpapiTestMailSender{})
	updatePermission := permission.MustDefinitionByCode("account.update.self")

	cookieUser := testutil.CreateUser(t, db, "email-cookie-old@test.com", "Password123", "EmailCookie", false)
	loginReq := httptest.NewRequest(http.MethodPost, "/v2/auth/login", strings.NewReader(`{"email":"email-cookie-old@test.com","password":"Password123"}`))
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("cookie login status=%d body=%q", loginRec.Code, loginRec.Body.String())
	}
	var accessCookie *http.Cookie
	for _, cookie := range loginRec.Result().Cookies() {
		if cookie.Name == "access_token" {
			accessCookie = cookie
		}
	}
	if accessCookie == nil {
		t.Fatal("cookie login did not return access_token")
	}
	cookieSend := httptest.NewRequest(http.MethodPost, "/v2/users/me/email/verification-code", strings.NewReader(`{"email":"email-cookie-new@test.com"}`))
	cookieSend.AddCookie(accessCookie)
	cookieSendRec := httptest.NewRecorder()
	router.ServeHTTP(cookieSendRec, cookieSend)
	if cookieSendRec.Code != http.StatusOK || !strings.Contains(cookieSendRec.Body.String(), `"ttl":300`) {
		t.Fatalf("cookie send status=%d body=%q", cookieSendRec.Code, cookieSendRec.Body.String())
	}
	cookieCode, err := cache.GetVerificationCode(t.Context(), "email-cookie-new@test.com", "email_change")
	if err != nil {
		t.Fatal(err)
	}
	cookieChange := httptest.NewRequest(http.MethodPut, "/v2/users/me/email", strings.NewReader(`{"email":"email-cookie-new@test.com","code":"`+cookieCode+`"}`))
	cookieChange.AddCookie(accessCookie)
	cookieChangeRec := httptest.NewRecorder()
	router.ServeHTTP(cookieChangeRec, cookieChange)
	if cookieChangeRec.Code != http.StatusNoContent || cookieChangeRec.Body.Len() != 0 {
		t.Fatalf("cookie change status=%d body=%q", cookieChangeRec.Code, cookieChangeRec.Body.String())
	}

	bearerUser := testutil.CreateUser(t, db, "email-bearer-old@test.com", "Password123", "EmailBearer", false)
	clientID := createActiveOAuthClientForAuthTest(t, db, bearerUser.ID, "email-bearer-client", []int64{int64(updatePermission.ID)})
	grantID := "grant-email-bearer"
	if err := db.OAuth.CreateGrant(t.Context(), model.OAuthGrant{
		ID: grantID, UserID: bearerUser.ID, SubjectID: permissiondb.SubjectIDForUser(bearerUser.ID),
		ClientID: clientID, Status: "active", CreatedAt: database.NowMS(),
	}, []int64{int64(updatePermission.ID)}); err != nil {
		t.Fatal(err)
	}
	rawToken := "email-change-bearer-token"
	now := database.NowMS()
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash: util.HashRefreshToken(rawToken), ClientID: clientID, UserID: bearerUser.ID, GrantID: grantID,
		PermissionIDs: []int64{int64(updatePermission.ID)}, ExpiresAt: now + int64(time.Hour/time.Millisecond), CreatedAt: now,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	bearerSend := httptest.NewRequest(http.MethodPost, "/v2/users/me/email/verification-code", strings.NewReader(`{"email":"email-bearer-new@test.com"}`))
	bearerSend.Header.Set("Authorization", "Bearer "+rawToken)
	bearerSendRec := httptest.NewRecorder()
	router.ServeHTTP(bearerSendRec, bearerSend)
	if bearerSendRec.Code != http.StatusOK || !strings.Contains(bearerSendRec.Body.String(), `"ttl":300`) {
		t.Fatalf("bearer send status=%d body=%q", bearerSendRec.Code, bearerSendRec.Body.String())
	}
	bearerCode, err := cache.GetVerificationCode(t.Context(), "email-bearer-new@test.com", "email_change")
	if err != nil {
		t.Fatal(err)
	}
	bearerChange := httptest.NewRequest(http.MethodPut, "/v2/users/me/email", strings.NewReader(`{"email":"email-bearer-new@test.com","code":"`+bearerCode+`"}`))
	bearerChange.Header.Set("Authorization", "Bearer "+rawToken)
	bearerChangeRec := httptest.NewRecorder()
	router.ServeHTTP(bearerChangeRec, bearerChange)
	if bearerChangeRec.Code != http.StatusNoContent || bearerChangeRec.Body.Len() != 0 {
		t.Fatalf("bearer change status=%d body=%q", bearerChangeRec.Code, bearerChangeRec.Body.String())
	}

	for _, expected := range []struct {
		id    string
		email string
	}{{cookieUser.ID, "email-cookie-new@test.com"}, {bearerUser.ID, "email-bearer-new@test.com"}} {
		updated, err := db.Users.GetByID(t.Context(), expected.id)
		if err != nil || updated == nil || updated.Email != expected.email {
			t.Fatalf("updated user id=%s user=%#v err=%v", expected.id, updated, err)
		}
	}
}

func TestAuthAcceptsClientBearerAndRejectsInactiveOrMissingBearerExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	owner := testutil.CreateUser(t, db, "auth-client-owner@test.com", "Password123", "AuthClientOwner", false)
	profileUser := testutil.CreateUser(t, db, "auth-client-profile@test.com", "Password123", "AuthClientProfile", false)
	profile := testutil.CreateProfile(t, db, profileUser.ID, "AuthBearerProfile", "default")
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	def := permission.MustDefinitionByCode("minecraft_session.hasjoined.server")
	clientID := createActiveOAuthClientForAuthTest(t, db, owner.ID, "auth-client-bearer", []int64{int64(def.ID)})
	if err := db.Permissions.SetPermissionOverrideForSubject(t.Context(), permissiondb.SubjectIDForClient(clientID), def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	rawToken := "client-bearer-token"
	now := database.NowMS()
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawToken),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(def.ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`{"username":"`+profile.Name+`","server_id":"route-server"}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"joined\":false,\"profile\":null}\n" {
		t.Fatalf("client bearer has-joined mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/users/me", nil)
	req.Header.Set("Authorization", "Bearer missing-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("missing bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	expired := "expired-client-bearer"
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(expired),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(def.ID)},
		ExpiresAt:     now - 1,
		CreatedAt:     now - int64(time.Hour/time.Millisecond),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v2/minecraft/session/has-joined", strings.NewReader(`{"username":"`+profile.Name+`","server_id":"route-server"}`))
	req.Header.Set("Authorization", "Bearer "+expired)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("expired bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func createActiveOAuthClientForAuthTest(t *testing.T, db *database.DB, ownerUserID, id string, permissionIDs []int64) string {
	t.Helper()
	now := database.NowMS()
	client := model.OAuthClient{
		ID:          id,
		OwnerUserID: ownerUserID,
		Name:        id,
		RedirectURI: "https://client.example/callback",
		ClientType:  "confidential",
		SecretHash:  util.HashRefreshToken("secret-" + id),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.OAuth.CreateClient(t.Context(), client, permissionIDs, nil); err != nil {
		t.Fatal(err)
	}
	return id
}

func containsAuthPermission(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type authCacheWriteFailStore struct {
	redisstore.Store
	setCalls int
}

type publicV2ResourceCase struct {
	name          string
	method        string
	path          string
	body          string
	guestStatus   int
	guestBody     string
	guestContains string
}

var publicV2ResourceCases = []publicV2ResourceCase{
	{name: "capabilities", method: http.MethodGet, path: "/v2/capabilities", guestStatus: http.StatusOK, guestContains: `"api_version":"v2"`},
	{name: "permission catalog", method: http.MethodGet, path: "/v2/permissions/catalog", guestStatus: http.StatusOK, guestContains: `"permissions":[`},
	{name: "skin library", method: http.MethodGet, path: "/v2/public/skin-library", guestStatus: http.StatusOK, guestContains: `"items":[]`},
	{name: "public settings", method: http.MethodGet, path: "/v2/public/settings", guestStatus: http.StatusOK, guestContains: `"site_name"`},
	{name: "homepage media", method: http.MethodGet, path: "/v2/public/homepage-media", guestStatus: http.StatusOK, guestBody: "{\"items\":[]}\n"},
	{name: "fallback status", method: http.MethodGet, path: "/v2/public/fallback-status", guestStatus: http.StatusOK, guestContains: `"endpoints":[]`},
	{name: "profile by name", method: http.MethodGet, path: "/v2/minecraft/profiles/by-name/Missing", guestStatus: http.StatusNotFound, guestBody: "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n"},
	{name: "profile by id", method: http.MethodGet, path: "/v2/minecraft/profiles/missing", guestStatus: http.StatusNotFound, guestBody: "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n"},
	{name: "textures property", method: http.MethodGet, path: "/v2/minecraft/profiles/missing/textures-property", guestStatus: http.StatusNotFound, guestBody: "{\"error\":{\"object\":\"minecraft_profile\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n"},
	{name: "profiles by names", method: http.MethodPost, path: "/v2/minecraft/profiles/by-names", body: `{"names":[]}`, guestStatus: http.StatusOK, guestBody: "{\"items\":[]}\n"},
}

var publicSiteAuthPaths = []string{"/", "/api/publickeys", "/api/publickeys/"}

func (s *authCacheWriteFailStore) SetAuthUser(context.Context, redisstore.AuthUser, time.Duration) error {
	s.setCalls++
	return errors.New("cache write failed")
}
