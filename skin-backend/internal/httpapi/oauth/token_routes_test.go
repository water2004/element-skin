package oauth_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/permission"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthClientCredentialsTokenWorksForMinecraftOnly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-client-route@test.com", "Password123", "OAuthClientRoute", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
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
	authMethods := stringSet(metadata["token_endpoint_auth_methods_supported"].([]any))
	if !authMethods["client_secret_basic"] || !authMethods["client_secret_post"] || !authMethods["none"] {
		t.Fatalf("metadata token auth methods mismatch: %#v", authMethods)
	}
	for _, key := range []string{
		"pushed_authorization_request_endpoint",
		"backchannel_authentication_endpoint",
		"dpop_signing_alg_values_supported",
		"introspection_endpoint_auth_methods_supported",
	} {
		if _, exists := metadata[key]; exists {
			t.Fatalf("metadata should omit unsupported field %q: %#v", key, metadata)
		}
	}
	protectedRes := doJSON(t, router, http.MethodGet, "/.well-known/oauth-protected-resource", nil, nil, "")
	if protectedRes.Code != http.StatusOK {
		t.Fatalf("protected resource metadata status=%d body=%s", protectedRes.Code, protectedRes.Body.String())
	}
	protected := decodeMap(t, protectedRes.Body.Bytes())
	if protected["resource"] != "http://localhost:8000/v2" ||
		len(protected["authorization_servers"].([]any)) != 1 ||
		protected["authorization_servers"].([]any)[0] != "http://localhost:8000" {
		t.Fatalf("protected resource metadata mismatch: %#v", protected)
	}
	if _, exists := protected["resource_signing_alg_values_supported"]; exists {
		t.Fatalf("protected resource metadata should omit unsupported signing algorithms: %#v", protected)
	}

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Minecraft server plugin",
		"redirect_uri": "https://server.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"minecraft_profile.read.public", "minecraft_session.hasjoined.server"},
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

	meRes := doJSON(t, router, http.MethodGet, "/v2/users/me", nil, nil, access)
	if meRes.Code != http.StatusForbidden || !strings.Contains(meRes.Body.String(), "permission denied") {
		t.Fatalf("app-only token should not read user me: status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	hasJoinedRes := doJSON(t, router, http.MethodPost, "/v2/minecraft/session/has-joined", map[string]any{
		"username":  "NoJoinedUser",
		"server_id": "missing-server",
	}, nil, access)
	if hasJoinedRes.Code != http.StatusOK || hasJoinedRes.Body.String() != "{\"joined\":false,\"profile\":null}\n" {
		t.Fatalf("app-only minecraft response mismatch: status=%d body=%s", hasJoinedRes.Code, hasJoinedRes.Body.String())
	}
}
