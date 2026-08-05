package oauth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthAuthorizationCodeFlowIssuesDelegatedBearerForV1API(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-flow@test.com", "Password123", "OAuthFlow", false)
	admin := testutil.CreateUser(t, db, "oauth-flow-admin@test.com", "Password123", "OAuthFlowAdmin", true, true)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)
	adminSession := webCookie(t, cfg.JWTSecret, admin.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
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
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", "https://client.example/wrong-callback")
	wrongRedirectRes := doForm(t, router, "/oauth/token", form, "", "")
	if wrongRedirectRes.Code != http.StatusBadRequest || wrongRedirectRes.Body.String() != "{\"error\":\"invalid_grant\",\"error_description\":\"invalid authorization code\"}\n" {
		t.Fatalf("wrong redirect token response mismatch: status=%d body=%s", wrongRedirectRes.Code, wrongRedirectRes.Body.String())
	}
	form.Set("redirect_uri", "https://client.example/callback")
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

	meRes := doJSON(t, router, http.MethodGet, "/v2/users/me", nil, nil, access)
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

	updateRes := doJSON(t, router, http.MethodPatch, "/v2/users/me", map[string]any{"display_name": "ShouldFail"}, nil, access)
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
	if reuseRes.Code != http.StatusBadRequest || reuseRes.Body.String() != "{\"error\":\"invalid_grant\",\"error_description\":\"invalid refresh_token\"}\n" {
		t.Fatalf("refresh reuse mismatch: status=%d body=%s", reuseRes.Code, reuseRes.Body.String())
	}
	grantsRes := doJSON(t, router, http.MethodGet, "/v2/oauth/grants?limit=10", nil, session, "")
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
	revokeGrantRes := doJSON(t, router, http.MethodDelete, "/v2/oauth/grants/"+grantID, nil, session, "")
	if revokeGrantRes.Code != http.StatusNoContent || revokeGrantRes.Body.Len() != 0 {
		t.Fatalf("revoke grant mismatch: status=%d body=%s", revokeGrantRes.Code, revokeGrantRes.Body.String())
	}
	revokeGrantAgainRes := doJSON(t, router, http.MethodDelete, "/v2/oauth/grants/"+grantID, nil, session, "")
	if revokeGrantAgainRes.Code != http.StatusNotFound || !strings.Contains(revokeGrantAgainRes.Body.String(), "oauth grant not found") {
		t.Fatalf("revoke grant replay mismatch: status=%d body=%s", revokeGrantAgainRes.Code, revokeGrantAgainRes.Body.String())
	}
}
