package oauth_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	oauthapi "element-skin/backend/internal/httpapi/oauth"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestOAuthAuthorizationCodeFlowIssuesDelegatedBearerForV1API(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-flow@test.com", "Password123", "OAuthFlow", false)
	admin := testutil.CreateUser(t, db, "oauth-flow-admin@test.com", "Password123", "OAuthFlowAdmin", true, true)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)
	adminSession := webCookie(t, cfg.JWTSecret, admin.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Flow app",
		"description":  "Flow app description",
		"redirect_uri": "https://client.example/callback",
		"website_url":  "https://client.example",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	app := decodeMap(t, createRes.Body.Bytes())
	clientID := app["client_id"].(string)
	clientSecret := app["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if clientID == "" || clientSecret == "" || app["secret_hash"] != nil {
		t.Fatalf("client response should expose id and one-time secret only: %#v", app)
	}
	if got := app["permissions"].([]any); len(got) != 1 || got[0] != "account.read.self" {
		t.Fatalf("client permissions mismatch: %#v", got)
	}

	verifier := "test-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	infoReq := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id="+url.QueryEscape(clientID)+
		"&redirect_uri="+url.QueryEscape("https://client.example/callback")+
		"&scope="+url.QueryEscape("account.read.self")+
		"&state=state-info&code_challenge="+url.QueryEscape(challenge)+
		"&code_challenge_method=S256", nil)
	infoReq.AddCookie(session)
	infoRec := httptest.NewRecorder()
	router.ServeHTTP(infoRec, infoReq)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("authorize info status=%d body=%s", infoRec.Code, infoRec.Body.String())
	}
	info := decodeMap(t, infoRec.Body.Bytes())
	if info["redirect_uri"] != "https://client.example/callback" || info["state"] != "state-info" ||
		len(info["scopes"].([]any)) != 1 || info["client"].(map[string]any)["client_id"] != clientID {
		t.Fatalf("authorize info mismatch: %#v", info)
	}
	authRes := doJSON(t, router, http.MethodPost, "/oauth/authorize", map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          "https://client.example/callback",
		"scope":                 "account.read.self",
		"state":                 "state-1",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}, session, "")
	if authRes.Code != http.StatusOK {
		t.Fatalf("authorize status=%d body=%s", authRes.Code, authRes.Body.String())
	}
	auth := decodeMap(t, authRes.Body.Bytes())
	code := auth["code"].(string)
	redirectURL := auth["redirect_url"].(string)
	if code == "" || !strings.Contains(redirectURL, "state=state-1") || !strings.Contains(redirectURL, "code=") {
		t.Fatalf("authorization response mismatch: %#v", auth)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", "https://client.example/callback")
	form.Set("code_verifier", verifier)
	tokenRes := doForm(t, router, "/oauth/token", form, "", "")
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}
	token := decodeMap(t, tokenRes.Body.Bytes())
	access := token["access_token"].(string)
	refresh := token["refresh_token"].(string)
	if access == "" || refresh == "" || token["token_type"] != "Bearer" || token["scope"] != "account.read.self" || token["expires_in"].(float64) != 3600 {
		t.Fatalf("token response mismatch: %#v", token)
	}

	meRes := doJSON(t, router, http.MethodGet, "/v1/users/me", nil, nil, access)
	if meRes.Code != http.StatusOK {
		t.Fatalf("bearer me status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	me := decodeMap(t, meRes.Body.Bytes())
	if me["id"] != user.ID {
		t.Fatalf("bearer me user mismatch: %#v", me)
	}
	permissions := stringSet(me["permissions"].([]any))
	if !permissions["account.read.self"] || permissions["account.update.self"] {
		t.Fatalf("delegated permissions should be narrowed exactly: %#v", permissions)
	}

	updateRes := doJSON(t, router, http.MethodPatch, "/v1/users/me", map[string]any{"display_name": "ShouldFail"}, nil, access)
	if updateRes.Code != http.StatusForbidden || !strings.Contains(updateRes.Body.String(), "permission denied") {
		t.Fatalf("unauthorized bearer update mismatch: status=%d body=%s", updateRes.Code, updateRes.Body.String())
	}

	refreshForm := url.Values{}
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("client_id", clientID)
	refreshForm.Set("client_secret", clientSecret)
	refreshForm.Set("refresh_token", refresh)
	refreshRes := doForm(t, router, "/oauth/token", refreshForm, "", "")
	if refreshRes.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshRes.Code, refreshRes.Body.String())
	}
	refreshed := decodeMap(t, refreshRes.Body.Bytes())
	if refreshed["access_token"] == access || refreshed["refresh_token"] == refresh || refreshed["scope"] != "account.read.self" {
		t.Fatalf("refresh should rotate tokens and preserve scope: %#v", refreshed)
	}
	introspectForm := url.Values{}
	introspectForm.Set("token", refreshed["access_token"].(string))
	introspectRes := doForm(t, router, "/oauth/introspect", introspectForm, adminSession.Value, "")
	if introspectRes.Code != http.StatusOK {
		t.Fatalf("introspect status=%d body=%s", introspectRes.Code, introspectRes.Body.String())
	}
	introspection := decodeMap(t, introspectRes.Body.Bytes())
	if introspection["active"] != true || introspection["client_id"] != clientID || introspection["user_id"] != user.ID ||
		introspection["scope"] != "account.read.self" {
		t.Fatalf("introspection mismatch: %#v", introspection)
	}
	revokeForm := url.Values{}
	revokeForm.Set("client_id", clientID)
	revokeForm.Set("client_secret", clientSecret)
	revokeForm.Set("token", refreshed["access_token"].(string))
	revokeRes := doForm(t, router, "/oauth/revoke", revokeForm, "", "")
	if revokeRes.Code != http.StatusOK || revokeRes.Body.String() != "" {
		t.Fatalf("revoke access mismatch: status=%d body=%s", revokeRes.Code, revokeRes.Body.String())
	}
	introspectRes = doForm(t, router, "/oauth/introspect", introspectForm, adminSession.Value, "")
	if introspectRes.Code != http.StatusOK || introspectRes.Body.String() != "{\"active\":false}\n" {
		t.Fatalf("inactive introspection mismatch: status=%d body=%s", introspectRes.Code, introspectRes.Body.String())
	}
	reuseRes := doForm(t, router, "/oauth/token", refreshForm, "", "")
	if reuseRes.Code != http.StatusBadRequest || !strings.Contains(reuseRes.Body.String(), "invalid refresh_token") {
		t.Fatalf("refresh reuse mismatch: status=%d body=%s", reuseRes.Code, reuseRes.Body.String())
	}
	grantsRes := doJSON(t, router, http.MethodGet, "/v1/oauth/grants?limit=10", nil, session, "")
	if grantsRes.Code != http.StatusOK {
		t.Fatalf("grant list status=%d body=%s", grantsRes.Code, grantsRes.Body.String())
	}
	grants := decodeMap(t, grantsRes.Body.Bytes())["items"].([]any)
	if len(grants) != 1 || grants[0].(map[string]any)["client_id"] != clientID ||
		grants[0].(map[string]any)["status"] != "active" {
		t.Fatalf("grant list mismatch: %#v", grants)
	}
	grantPermissions := grants[0].(map[string]any)["permissions"].([]any)
	if len(grantPermissions) != 1 || grantPermissions[0] != "account.read.self" {
		t.Fatalf("grant permission list mismatch: %#v", grantPermissions)
	}
	grantID := grants[0].(map[string]any)["id"].(string)
	revokeGrantRes := doJSON(t, router, http.MethodDelete, "/v1/oauth/grants/"+grantID, nil, session, "")
	if revokeGrantRes.Code != http.StatusOK || revokeGrantRes.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("revoke grant mismatch: status=%d body=%s", revokeGrantRes.Code, revokeGrantRes.Body.String())
	}
	revokeGrantAgainRes := doJSON(t, router, http.MethodDelete, "/v1/oauth/grants/"+grantID, nil, session, "")
	if revokeGrantAgainRes.Code != http.StatusNotFound || !strings.Contains(revokeGrantAgainRes.Body.String(), "oauth grant not found") {
		t.Fatalf("revoke grant replay mismatch: status=%d body=%s", revokeGrantAgainRes.Code, revokeGrantAgainRes.Body.String())
	}
}

func TestOAuthAppManagementRoutesCoverReviewSecretListsAndDelete(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	owner := testutil.CreateUser(t, db, "oauth-app-owner@test.com", "Password123", "OAuthAppOwner", false)
	admin := testutil.CreateUser(t, db, "oauth-app-admin@test.com", "Password123", "OAuthAppAdmin", true, true)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	ownerSession := webCookie(t, cfg.JWTSecret, owner.ID)
	adminSession := webCookie(t, cfg.JWTSecret, admin.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Route managed app",
		"description":  "Route app description",
		"redirect_uri": "https://route.example/callback",
		"website_url":  "https://route.example",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self", "account.update.self"},
	}, ownerSession, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	app := decodeMap(t, createRes.Body.Bytes())
	clientID := app["client_id"].(string)
	firstSecret := app["client_secret"].(string)
	if clientID == "" || firstSecret == "" || app["status"] != "pending" {
		t.Fatalf("created app mismatch: %#v", app)
	}

	listRes := doJSON(t, router, http.MethodGet, "/v1/oauth/apps?limit=5", nil, ownerSession, "")
	if listRes.Code != http.StatusOK {
		t.Fatalf("list apps status=%d body=%s", listRes.Code, listRes.Body.String())
	}
	list := decodeMap(t, listRes.Body.Bytes())["items"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["client_id"] != clientID {
		t.Fatalf("owned app list mismatch: %#v", list)
	}
	getRes := doJSON(t, router, http.MethodGet, "/v1/oauth/apps/"+clientID, nil, ownerSession, "")
	if getRes.Code != http.StatusOK {
		t.Fatalf("get app status=%d body=%s", getRes.Code, getRes.Body.String())
	}
	got := decodeMap(t, getRes.Body.Bytes())
	if got["client_id"] != clientID || got["name"] != "Route managed app" {
		t.Fatalf("get app mismatch: %#v", got)
	}
	updateRes := doJSON(t, router, http.MethodPatch, "/v1/oauth/apps/"+clientID, map[string]any{
		"name":         "Route managed app updated",
		"description":  "Updated route description",
		"redirect_uri": "https://route.example/new-callback",
		"website_url":  "https://route.example/docs",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
		"status":       "active",
	}, ownerSession, "")
	if updateRes.Code != http.StatusOK {
		t.Fatalf("update app status=%d body=%s", updateRes.Code, updateRes.Body.String())
	}
	updated := decodeMap(t, updateRes.Body.Bytes())
	if updated["name"] != "Route managed app updated" || updated["status"] != "pending" ||
		updated["redirect_uri"] != "https://route.example/new-callback" {
		t.Fatalf("owner update app mismatch: %#v", updated)
	}
	submitRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps/"+clientID+"/review-submission", nil, ownerSession, "")
	if submitRes.Code != http.StatusOK || decodeMap(t, submitRes.Body.Bytes())["status"] != "pending" {
		t.Fatalf("submit app mismatch: status=%d body=%s", submitRes.Code, submitRes.Body.String())
	}
	adminListRes := doJSON(t, router, http.MethodGet, "/v1/admin/oauth/apps?status=pending&limit=10", nil, adminSession, "")
	if adminListRes.Code != http.StatusOK {
		t.Fatalf("admin list status=%d body=%s", adminListRes.Code, adminListRes.Body.String())
	}
	adminItems := decodeMap(t, adminListRes.Body.Bytes())["items"].([]any)
	if len(adminItems) != 1 || adminItems[0].(map[string]any)["client_id"] != clientID {
		t.Fatalf("admin pending list mismatch: %#v", adminItems)
	}
	adminSummary := adminItems[0].(map[string]any)
	if _, ok := adminSummary["permissions"]; ok {
		t.Fatalf("admin list must not include permissions: %#v", adminSummary)
	}
	if _, ok := adminSummary["redirect_uri"]; ok {
		t.Fatalf("admin list must not include redirect_uri: %#v", adminSummary)
	}
	if adminSummary["name"] != "Route managed app updated" || adminSummary["status"] != "pending" ||
		adminSummary["client_type"] != "confidential" {
		t.Fatalf("admin summary fields mismatch: %#v", adminSummary)
	}
	adminDetailRes := doJSON(t, router, http.MethodGet, "/v1/admin/oauth/apps/"+clientID, nil, adminSession, "")
	if adminDetailRes.Code != http.StatusOK {
		t.Fatalf("admin detail status=%d body=%s", adminDetailRes.Code, adminDetailRes.Body.String())
	}
	adminDetail := decodeMap(t, adminDetailRes.Body.Bytes())
	if adminDetail["client_id"] != clientID ||
		adminDetail["redirect_uri"] != "https://route.example/new-callback" ||
		adminDetail["website_url"] != "https://route.example/docs" {
		t.Fatalf("admin detail fields mismatch: %#v", adminDetail)
	}
	adminPermissions := adminDetail["permissions"].([]any)
	if len(adminPermissions) != 1 || adminPermissions[0] != "account.read.self" {
		t.Fatalf("admin detail permissions mismatch: %#v", adminPermissions)
	}
	reviewRes := doJSON(t, router, http.MethodPatch, "/v1/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "active"}, adminSession, "")
	if reviewRes.Code != http.StatusOK || decodeMap(t, reviewRes.Body.Bytes())["status"] != "active" {
		t.Fatalf("review app mismatch: status=%d body=%s", reviewRes.Code, reviewRes.Body.String())
	}
	secretRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps/"+clientID+"/secret", nil, ownerSession, "")
	if secretRes.Code != http.StatusOK {
		t.Fatalf("rotate secret status=%d body=%s", secretRes.Code, secretRes.Body.String())
	}
	rotated := decodeMap(t, secretRes.Body.Bytes())
	if rotated["client_secret"] == "" || rotated["client_secret"] == firstSecret || rotated["status"] != "active" {
		t.Fatalf("rotated secret mismatch: %#v", rotated)
	}
	deleteRes := doJSON(t, router, http.MethodDelete, "/v1/oauth/apps/"+clientID, nil, ownerSession, "")
	if deleteRes.Code != http.StatusOK || deleteRes.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete app mismatch: status=%d body=%s", deleteRes.Code, deleteRes.Body.String())
	}
	missingRes := doJSON(t, router, http.MethodGet, "/v1/oauth/apps/"+clientID, nil, ownerSession, "")
	if missingRes.Code != http.StatusNotFound || !strings.Contains(missingRes.Body.String(), "oauth client not found") {
		t.Fatalf("deleted get mismatch: status=%d body=%s", missingRes.Code, missingRes.Body.String())
	}
}

func TestOAuthCreateAppRejectsScopeMissingFromActor(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-scope-deny@test.com", "Password123", "OAuthScopeDeny", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	res := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Denied app",
		"redirect_uri": "https://client.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"account.ban.any"},
	}, webCookie(t, cfg.JWTSecret, user.ID), "")
	if res.Code != http.StatusForbidden || !strings.Contains(res.Body.String(), "permission denied") {
		t.Fatalf("scope deny mismatch: status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestOAuthClientCredentialsTokenWorksForMinecraftOnly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-client-route@test.com", "Password123", "OAuthClientRoute", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)

	metadataRes := doJSON(t, router, http.MethodGet, "/.well-known/oauth-authorization-server", nil, nil, "")
	if metadataRes.Code != http.StatusOK {
		t.Fatalf("metadata status=%d body=%s", metadataRes.Code, metadataRes.Body.String())
	}
	metadata := decodeMap(t, metadataRes.Body.Bytes())
	grants := stringSet(metadata["grant_types_supported"].([]any))
	if !grants["authorization_code"] || !grants["refresh_token"] || !grants["client_credentials"] {
		t.Fatalf("metadata grant types mismatch: %#v", grants)
	}
	protectedRes := doJSON(t, router, http.MethodGet, "/.well-known/oauth-protected-resource", nil, nil, "")
	if protectedRes.Code != http.StatusOK {
		t.Fatalf("protected resource metadata status=%d body=%s", protectedRes.Code, protectedRes.Body.String())
	}
	protected := decodeMap(t, protectedRes.Body.Bytes())
	if protected["resource"] != "http://localhost:8000/v1" ||
		len(protected["authorization_servers"].([]any)) != 1 ||
		protected["authorization_servers"].([]any)[0] != "http://localhost:8000" {
		t.Fatalf("protected resource metadata mismatch: %#v", protected)
	}

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Minecraft server plugin",
		"redirect_uri": "https://server.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"minecraft_profile.read.public"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	app := decodeMap(t, createRes.Body.Bytes())
	clientID := app["client_id"].(string)
	clientSecret := app["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if err := db.Permissions.SetPermissionOverrideForSubject(
		t.Context(),
		permissiondb.SubjectIDForClient(clientID),
		permission.MustDefinitionByCode("minecraft_session.hasjoined.server"),
		"allow",
		"",
	); err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "minecraft_session.hasjoined.server")
	tokenRes := doForm(t, router, "/oauth/token", form, "", "")
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("client credentials token status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}
	token := decodeMap(t, tokenRes.Body.Bytes())
	access := token["access_token"].(string)
	if access == "" || token["refresh_token"] != nil || token["scope"] != "minecraft_session.hasjoined.server" {
		t.Fatalf("client credentials token response mismatch: %#v", token)
	}
	basicForm := url.Values{}
	basicForm.Set("grant_type", "client_credentials")
	basicForm.Set("scope", "minecraft_session.hasjoined.server")
	basicTokenRes := doFormBasic(t, router, "/oauth/token", basicForm, clientID, clientSecret)
	if basicTokenRes.Code != http.StatusOK {
		t.Fatalf("basic client credentials token status=%d body=%s", basicTokenRes.Code, basicTokenRes.Body.String())
	}
	basicToken := decodeMap(t, basicTokenRes.Body.Bytes())
	if basicToken["access_token"] == "" || basicToken["scope"] != "minecraft_session.hasjoined.server" ||
		basicToken["refresh_token"] != nil {
		t.Fatalf("basic client credentials token mismatch: %#v", basicToken)
	}

	meRes := doJSON(t, router, http.MethodGet, "/v1/users/me", nil, nil, access)
	if meRes.Code != http.StatusForbidden || !strings.Contains(meRes.Body.String(), "permission denied") {
		t.Fatalf("app-only token should not read user me: status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	hasJoinedRes := doJSON(t, router, http.MethodPost, "/v1/minecraft/session/has-joined", map[string]any{
		"username":  "NoJoinedUser",
		"server_id": "missing-server",
	}, nil, access)
	if hasJoinedRes.Code != http.StatusOK || hasJoinedRes.Body.String() != "{\"joined\":false,\"profile\":null}\n" {
		t.Fatalf("app-only minecraft response mismatch: status=%d body=%s", hasJoinedRes.Code, hasJoinedRes.Body.String())
	}
}

func TestOAuthDeviceCodeFlowRoutesIssueDelegatedBearer(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	user := testutil.CreateUser(t, db, "oauth-device-route@test.com", "Password123", "OAuthDeviceRoute", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Device route app",
		"redirect_uri": "https://device.example/callback",
		"client_type":  "public",
		"permissions":  []string{"account.read.self"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)
	activateOAuthClient(t, db, clientID)

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", "account.read.self")
	deviceRes := doForm(t, router, "/oauth/device/code", form, "", "")
	if deviceRes.Code != http.StatusOK {
		t.Fatalf("device code status=%d body=%s", deviceRes.Code, deviceRes.Body.String())
	}
	device := decodeMap(t, deviceRes.Body.Bytes())
	deviceCode := device["device_code"].(string)
	userCode := device["user_code"].(string)
	if deviceCode == "" || userCode == "" || device["verification_uri"] != "https://skin.example/oauth/device" ||
		device["verification_uri_complete"] != "https://skin.example/oauth/device?user_code="+userCode ||
		device["scope"] != "account.read.self" {
		t.Fatalf("device response mismatch: %#v", device)
	}

	infoReq := httptest.NewRequest(http.MethodGet, "/oauth/device?user_code="+userCode, nil)
	infoReq.AddCookie(session)
	infoRec := httptest.NewRecorder()
	router.ServeHTTP(infoRec, infoReq)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("device info status=%d body=%s", infoRec.Code, infoRec.Body.String())
	}
	info := decodeMap(t, infoRec.Body.Bytes())
	if info["status"] != "pending" || len(info["scopes"].([]any)) != 1 {
		t.Fatalf("device info mismatch: %#v", info)
	}

	decisionRes := doJSON(t, router, http.MethodPost, "/oauth/device", map[string]any{
		"user_code": userCode,
		"approve":   true,
	}, session, "")
	if decisionRes.Code != http.StatusOK || decisionRes.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("device decision mismatch: status=%d body=%s", decisionRes.Code, decisionRes.Body.String())
	}
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	tokenForm.Set("client_id", clientID)
	tokenForm.Set("device_code", deviceCode)
	tokenRes := doForm(t, router, "/oauth/token", tokenForm, "", "")
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("device token status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}
	token := decodeMap(t, tokenRes.Body.Bytes())
	access := token["access_token"].(string)
	if access == "" || token["refresh_token"].(string) == "" || token["scope"] != "account.read.self" {
		t.Fatalf("device token mismatch: %#v", token)
	}
	meRes := doJSON(t, router, http.MethodGet, "/v1/users/me", nil, nil, access)
	if meRes.Code != http.StatusOK {
		t.Fatalf("device bearer me status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	me := decodeMap(t, meRes.Body.Bytes())
	if me["id"] != user.ID {
		t.Fatalf("device bearer user mismatch: %#v", me)
	}
}

func TestOAuthClientPermissionRoutesManageClientSubjectExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	admin := testutil.CreateUser(t, db, "oauth-client-permission@test.com", "Password123", "OAuthClientPermission", true, true)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, admin.ID)
	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Permission route app",
		"redirect_uri": "https://client.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"minecraft_profile.read.public"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)
	grantRes := doJSON(t, router, http.MethodPut, "/v1/oauth/apps/"+clientID+"/permissions/minecraft_session.hasjoined.server", map[string]any{
		"effect": "allow",
	}, session, "")
	if grantRes.Code != http.StatusOK || grantRes.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("grant client permission mismatch: status=%d body=%s", grantRes.Code, grantRes.Body.String())
	}
	permissionsRes := doJSON(t, router, http.MethodGet, "/v1/oauth/apps/"+clientID+"/permissions", nil, session, "")
	if permissionsRes.Code != http.StatusOK {
		t.Fatalf("client permissions status=%d body=%s", permissionsRes.Code, permissionsRes.Body.String())
	}
	body := decodeMap(t, permissionsRes.Body.Bytes())
	if body["subject_id"] != permissiondb.SubjectIDForClient(clientID) {
		t.Fatalf("client permission subject mismatch: %#v", body)
	}
	effective := stringSet(body["effective_permissions"].([]any))
	if !effective["minecraft_session.hasjoined.server"] {
		t.Fatalf("effective client permissions missing grant: %#v", effective)
	}
	overrides := body["overrides"].([]any)
	if len(overrides) != 1 || overrides[0].(map[string]any)["permission_code"] != "minecraft_session.hasjoined.server" ||
		overrides[0].(map[string]any)["effect"] != "allow" {
		t.Fatalf("client permission overrides mismatch: %#v", overrides)
	}
	clearRes := doJSON(t, router, http.MethodDelete, "/v1/oauth/apps/"+clientID+"/permissions/minecraft_session.hasjoined.server", nil, session, "")
	if clearRes.Code != http.StatusOK || clearRes.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("clear client permission mismatch: status=%d body=%s", clearRes.Code, clearRes.Body.String())
	}
}

func TestOAuthRoutesRejectMalformedInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	adminUser := testutil.CreateUser(t, db, "oauth-route-errors-admin@test.com", "Password123", "OAuthRouteErrorsAdmin", true, true)
	user := testutil.CreateUser(t, db, "oauth-route-errors-user@test.com", "Password123", "OAuthRouteErrorsUser", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	adminSession := webCookie(t, cfg.JWTSecret, adminUser.ID)
	userSession := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Malformed route app",
		"redirect_uri": "https://malformed.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
	}, userSession, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create route error app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		cookie *http.Cookie
	}{
		{name: "create app", method: http.MethodPost, path: "/v1/oauth/apps", cookie: userSession},
		{name: "update app", method: http.MethodPatch, path: "/v1/oauth/apps/" + clientID, cookie: userSession},
		{name: "review app", method: http.MethodPatch, path: "/v1/admin/oauth/apps/" + clientID + "/review", cookie: adminSession},
		{name: "permission override", method: http.MethodPut, path: "/v1/oauth/apps/" + clientID + "/permissions/account.read.self", cookie: adminSession},
		{name: "authorize", method: http.MethodPost, path: "/oauth/authorize", cookie: userSession},
		{name: "device decision", method: http.MethodPost, path: "/oauth/device", cookie: userSession},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRaw(t, router, tc.method, tc.path, "{bad", "application/json", tc.cookie, "")
			if res.Code != http.StatusBadRequest || res.Body.String() != "{\"detail\":\"invalid json\"}\n" {
				t.Fatalf("%s invalid json mismatch: status=%d body=%s", tc.name, res.Code, res.Body.String())
			}
		})
	}

	reviewRes := doJSON(t, router, http.MethodPatch, "/v1/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "pending"}, adminSession, "")
	if reviewRes.Code != http.StatusBadRequest || reviewRes.Body.String() != "{\"detail\":\"invalid status\"}\n" {
		t.Fatalf("invalid review status mismatch: status=%d body=%s", reviewRes.Code, reviewRes.Body.String())
	}
	rejectWithoutReason := doJSON(t, router, http.MethodPatch, "/v1/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "rejected"}, adminSession, "")
	if rejectWithoutReason.Code != http.StatusBadRequest || rejectWithoutReason.Body.String() != "{\"detail\":\"reason is required\"}\n" {
		t.Fatalf("reject without reason mismatch: status=%d body=%s", rejectWithoutReason.Code, rejectWithoutReason.Body.String())
	}
	grantRes := doJSON(t, router, http.MethodPut, "/v1/oauth/apps/"+clientID+"/permissions/nope.nope.nope", map[string]any{"effect": "allow"}, adminSession, "")
	if grantRes.Code != http.StatusBadRequest || grantRes.Body.String() != "{\"detail\":\"invalid permission\"}\n" {
		t.Fatalf("invalid client permission mismatch: status=%d body=%s", grantRes.Code, grantRes.Body.String())
	}
	clearRes := doJSON(t, router, http.MethodDelete, "/v1/oauth/apps/"+clientID+"/permissions/nope.nope.nope", nil, adminSession, "")
	if clearRes.Code != http.StatusBadRequest || clearRes.Body.String() != "{\"detail\":\"invalid permission\"}\n" {
		t.Fatalf("clear invalid client permission mismatch: status=%d body=%s", clearRes.Code, clearRes.Body.String())
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "device code", path: "/oauth/device/code"},
		{name: "token", path: "/oauth/token"},
		{name: "revoke", path: "/oauth/revoke"},
		{name: "introspect", path: "/oauth/introspect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRaw(t, router, http.MethodPost, tc.path, "%zz", "application/x-www-form-urlencoded", adminSession, "")
			if res.Code != http.StatusBadRequest || res.Body.String() != "{\"detail\":\"invalid form\"}\n" {
				t.Fatalf("%s invalid form mismatch: status=%d body=%s", tc.name, res.Code, res.Body.String())
			}
		})
	}
}

func TestOAuthHandlerForwardsServiceErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	cfg.APIURL = ""
	cfg.SiteURL = "https://skin.example"
	handler := oauthapi.New(cfg, db, redisstore.NewMemoryStore(), nil)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "list apps", method: http.MethodGet, path: "/v1/oauth/apps", status: http.StatusForbidden, body: "{\"detail\":\"permission denied\"}\n", call: handler.ListApps},
		{name: "admin list apps", method: http.MethodGet, path: "/v1/admin/oauth/apps", status: http.StatusForbidden, body: "{\"detail\":\"permission denied\"}\n", call: handler.ListAdminApps},
		{name: "submit review", method: http.MethodPost, path: "/v1/oauth/apps/missing/review-submission", status: http.StatusNotFound, body: "{\"detail\":\"oauth client not found\"}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.SubmitAppReview(rec, req)
		}},
		{name: "rotate secret", method: http.MethodPost, path: "/v1/oauth/apps/missing/secret", status: http.StatusNotFound, body: "{\"detail\":\"oauth client not found\"}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.RotateSecret(rec, req)
		}},
		{name: "delete app", method: http.MethodDelete, path: "/v1/oauth/apps/missing", status: http.StatusNotFound, body: "{\"detail\":\"oauth client not found\"}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.DeleteApp(rec, req)
		}},
		{name: "client permissions", method: http.MethodGet, path: "/v1/oauth/apps/missing/permissions", status: http.StatusForbidden, body: "{\"detail\":\"permission denied\"}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.ClientPermissions(rec, req)
		}},
		{name: "list grants", method: http.MethodGet, path: "/v1/oauth/grants", status: http.StatusForbidden, body: "{\"detail\":\"permission denied\"}\n", call: handler.ListGrants},
		{name: "authorize info", method: http.MethodGet, path: "/oauth/authorize", status: http.StatusBadRequest, body: "{\"detail\":\"response_type must be code\"}\n", call: handler.AuthorizeInfo},
		{name: "device info", method: http.MethodGet, path: "/oauth/device?user_code=missing", status: http.StatusForbidden, body: "{\"detail\":\"permission denied\"}\n", call: handler.DeviceInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != tc.status || rec.Body.String() != tc.body {
				t.Fatalf("%s service error mismatch: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	handler.ProtectedResourceMetadata(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("site-url protected metadata status=%d body=%s", rec.Code, rec.Body.String())
	}
	protected := decodeMap(t, rec.Body.Bytes())
	if protected["resource"] != "https://skin.example/v1" ||
		protected["authorization_servers"].([]any)[0] != "https://skin.example" {
		t.Fatalf("site-url protected metadata mismatch: %#v", protected)
	}
}

func webCookie(t *testing.T, secret, userID string) *http.Cookie {
	t.Helper()
	token, err := util.CreateAccessToken(secret, userID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "access_token", Value: token}
}

func activateOAuthClient(t *testing.T, db *database.DB, clientID string) {
	t.Helper()
	if ok, err := db.OAuth.UpdateClientStatus(t.Context(), clientID, "active", database.NowMS()); err != nil || !ok {
		t.Fatalf("activate oauth client: ok=%v err=%v", ok, err)
	}
}

func doJSON(t *testing.T, router http.Handler, method, path string, body any, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doRaw(t *testing.T, router http.Handler, method, path, body, contentType string, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doForm(t *testing.T, router http.Handler, path string, form url.Values, cookieValue, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: cookieValue})
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doFormBasic(t *testing.T, router http.Handler, path string, form url.Values, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(username, password)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode json %q: %v", string(data), err)
	}
	return out
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func stringSet(values []any) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		if s, ok := value.(string); ok {
			out[s] = true
		}
	}
	return out
}
