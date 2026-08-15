package identity_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	identityapi "element-skin/backend/internal/httpapi/identity"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	identitysvc "element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/golang-jwt/jwt/v5"
)

func TestProviderRoutesUseExactV2ContractsAndNeverExposeClientSecret(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	var discoveryServer *httptest.Server
	discoveryServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"userinfo_endpoint":%q,"jwks_uri":%q}`,
			discoveryServer.URL, discoveryServer.URL+"/authorize", discoveryServer.URL+"/token",
			discoveryServer.URL+"/userinfo", discoveryServer.URL+"/jwks")
	}))
	defer discoveryServer.Close()
	h := identityapi.New(cfg, db, testutil.NewMemoryRedis(), nil)
	admin := testutil.CreateUser(t, db, "identity-route-admin@example.com", "Password123", "IdentityRouteAdmin", true)
	adminActor := identityRouteActor(t, db, admin.ID)

	requestBody := fmt.Sprintf(`{"name":"Example Accounts","issuer_url":%q,"client_id":"client-id","client_secret":"super-secret","scopes":["profile email"],"adapter":"generic_oidc","icon_url":"https://accounts.example/icon.png","enabled":true,"login_enabled":true,"link_enabled":true,"display_order":7}`, discoveryServer.URL)
	req := identityRouteRequest(http.MethodPost, "/v2/admin/identity-providers", requestBody, adminActor)
	rec := httptest.NewRecorder()
	h.CreateProvider(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create provider status=%d body=%q", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	providerID, _ := created["id"].(string)
	if providerID == "" || len(created) != 18 || created["name"] != "Example Accounts" ||
		created["issuer_url"] != discoveryServer.URL || created["client_id"] != "client-id" ||
		created["has_client_secret"] != true || created["enabled"] != true || created["display_order"] != float64(7) ||
		strings.Contains(rec.Body.String(), "super-secret") || strings.Contains(rec.Body.String(), "client_secret_ciphertext") {
		t.Fatalf("create provider response mismatch: %#v", created)
	}
	if scopes, ok := created["scopes"].([]any); !ok || len(scopes) != 3 || scopes[0] != "email" || scopes[1] != "openid" || scopes[2] != "profile" {
		t.Fatalf("normalized scopes mismatch: %#v", created["scopes"])
	}

	req = identityRouteRequest(http.MethodGet, "/v2/auth/identity-providers", "", permission.GuestActor())
	rec = httptest.NewRecorder()
	h.PublicProviders(rec, req)
	wantPublic := `{"items":[{"adapter":"generic_oidc","icon_url":"https://accounts.example/icon.png","id":"` + providerID + `","link_enabled":true,"login_enabled":true,"name":"Example Accounts"}],"redirect_uri":"http://localhost:8000/v2/auth/oidc/callback"}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != wantPublic {
		t.Fatalf("public provider response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodGet, "/v2/admin/identity-providers", "", adminActor)
	rec = httptest.NewRecorder()
	h.ListProviders(rec, req)
	var listed struct {
		Items       []map[string]any `json:"items"`
		RedirectURI string           `json:"redirect_uri"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(listed.Items) != 1 || listed.Items[0]["id"] != providerID ||
		listed.Items[0]["has_client_secret"] != true || len(listed.Items[0]) != 18 ||
		listed.RedirectURI != "http://localhost:8000/v2/auth/oidc/callback" {
		t.Fatalf("admin provider list mismatch: status=%d response=%#v", rec.Code, listed)
	}

	req = identityRouteRequest(http.MethodGet, "/v2/admin/identity-providers/"+providerID, "", adminActor)
	req.SetPathValue("provider_id", providerID)
	rec = httptest.NewRecorder()
	h.GetProvider(rec, req)
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || detail["id"] != providerID || detail["name"] != "Example Accounts" ||
		detail["has_client_secret"] != true || len(detail) != 18 {
		t.Fatalf("provider detail mismatch: status=%d response=%#v", rec.Code, detail)
	}

	updateBody := fmt.Sprintf(`{"name":"Updated Accounts","issuer_url":%q,"client_id":"client-id","scopes":["openid","groups"],"adapter":"generic_oidc","icon_url":"https://accounts.example/updated.png","enabled":true,"login_enabled":true,"link_enabled":false,"display_order":8}`, discoveryServer.URL)
	req = identityRouteRequest(http.MethodPut, "/v2/admin/identity-providers/"+providerID, updateBody, adminActor)
	req.SetPathValue("provider_id", providerID)
	rec = httptest.NewRecorder()
	h.UpdateProvider(rec, req)
	var updated map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || updated["id"] != providerID || updated["name"] != "Updated Accounts" ||
		updated["link_enabled"] != false || updated["display_order"] != float64(8) ||
		updated["has_client_secret"] != true || len(updated) != 18 {
		t.Fatalf("provider update mismatch: status=%d response=%#v", rec.Code, updated)
	}
	if scopes, ok := updated["scopes"].([]any); !ok || len(scopes) != 2 || scopes[0] != "groups" || scopes[1] != "openid" {
		t.Fatalf("updated scopes mismatch: %#v", updated["scopes"])
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/admin/identity-providers/"+providerID, "", adminActor)
	req.SetPathValue("provider_id", providerID)
	rec = httptest.NewRecorder()
	h.DeleteProvider(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete provider response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if provider, err := db.Identities.GetProvider(req.Context(), providerID); err != nil || provider != nil {
		t.Fatalf("deleted provider remains: provider=%#v err=%v", provider, err)
	}
}

func TestIdentityRoutesListUpdateDeleteOnlyOwnedRecordsWithExactErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := identityapi.New(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)
	owner := testutil.CreateUser(t, db, "identity-owner@example.com", "Password123", "IdentityOwner", false)
	other := testutil.CreateUser(t, db, "identity-other@example.com", "Password123", "IdentityOther", false)
	provider := model.IdentityProvider{
		ID: "identity-route-provider", Name: "Route Provider", IssuerURL: "https://route.example",
		AuthorizationEndpoint: "https://route.example/authorize", TokenEndpoint: "https://route.example/token",
		JWKSURI: "https://route.example/jwks", ClientID: "client", Scopes: []string{"openid"},
		Adapter: identitysvc.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(t.Context(), provider); err != nil {
		t.Fatal(err)
	}
	for _, item := range []model.ExternalIdentity{
		{ID: "identity-a", UserID: owner.ID, ProviderID: provider.ID, Subject: "subject-a", Label: "first", Email: "a@remote.example", EmailVerified: true, DisplayName: "Remote A", CreatedAt: 10, UpdatedAt: 11},
		{ID: "identity-b", UserID: owner.ID, ProviderID: provider.ID, Subject: "subject-b", Label: "second", Email: "b@remote.example", DisplayName: "Remote B", CreatedAt: 20, UpdatedAt: 21},
	} {
		if err := db.Identities.CreateIdentity(t.Context(), item, model.ExternalIdentityCredential{IdentityID: item.ID, GrantedScopes: []string{}, UpdatedAt: item.UpdatedAt}); err != nil {
			t.Fatal(err)
		}
	}
	if updated, err := db.Identities.MarkCredentialRefreshRejected(t.Context(), "identity-b", 22); err != nil || !updated {
		t.Fatalf("mark identity-b reauthorization state updated=%v err=%v", updated, err)
	}
	ownerActor := identityRouteActor(t, db, owner.ID)
	otherActor := identityRouteActor(t, db, other.ID)

	req := identityRouteRequest(http.MethodGet, "/v2/users/me/identities", "", ownerActor)
	rec := httptest.NewRecorder()
	h.ListIdentities(rec, req)
	wantList := `{"items":[{"authorization_status":"active","avatar_url":"","created_at":10,"display_name":"Remote A","email":"a@remote.example","email_verified":true,"id":"identity-a","label":"first","last_login_at":null,"last_refresh_at":null,"last_refresh_error_at":null,"provider_adapter":"generic_oidc","provider_enabled":true,"provider_icon_url":"","provider_id":"identity-route-provider","provider_link_enabled":true,"provider_name":"Route Provider","subject":"subject-a","updated_at":11},{"authorization_status":"reauthorization_required","avatar_url":"","created_at":20,"display_name":"Remote B","email":"b@remote.example","email_verified":false,"id":"identity-b","label":"second","last_login_at":null,"last_refresh_at":null,"last_refresh_error_at":22,"provider_adapter":"generic_oidc","provider_enabled":true,"provider_icon_url":"","provider_id":"identity-route-provider","provider_link_enabled":true,"provider_name":"Route Provider","subject":"subject-b","updated_at":21}]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != wantList {
		t.Fatalf("identity list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodPatch, "/v2/users/me/identities/identity-a", `{"label":"  primary  "}`, ownerActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.UpdateIdentity(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("identity update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Identities.GetIdentity(t.Context(), "identity-a")
	if err != nil || updated == nil || updated.Label != "primary" {
		t.Fatalf("identity label state mismatch: identity=%#v err=%v", updated, err)
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/users/me/identities/identity-a", "", otherActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.DeleteIdentity(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"external identity not found\"}\n" {
		t.Fatalf("cross-account delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if item, err := db.Identities.GetIdentity(t.Context(), "identity-a"); err != nil || item == nil {
		t.Fatalf("cross-account delete mutated identity: identity=%#v err=%v", item, err)
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/users/me/identities/identity-a", "", ownerActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.DeleteIdentity(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("owned identity delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if item, err := db.Identities.GetIdentity(t.Context(), "identity-a"); err != nil || item != nil {
		t.Fatalf("owned identity was not deleted: identity=%#v err=%v", item, err)
	}
}

func TestIdentityRoutesRejectInvalidJSONAndMissingPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := identityapi.New(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)

	req := identityRouteRequest(http.MethodPost, "/v2/admin/identity-providers", `{`, permission.Actor{})
	rec := httptest.NewRecorder()
	h.CreateProvider(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("provider invalid JSON response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodGet, "/v2/users/me/identities", "", permission.Actor{})
	rec = httptest.NewRecorder()
	h.ListIdentities(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
		t.Fatalf("identity permission response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{name: "start authorization invalid json", call: h.StartAuthorization, req: identityRouteRequest(http.MethodPost, "/v2/identity-authorizations", `{`, permission.GuestActor())},
		{name: "update provider invalid json", call: h.UpdateProvider, req: identityRouteRequest(http.MethodPut, "/v2/admin/identity-providers/missing", `{`, permission.Actor{})},
		{name: "update identity invalid json", call: h.UpdateIdentity, req: identityRouteRequest(http.MethodPatch, "/v2/users/me/identities/missing", `{`, permission.Actor{})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, tc.req)
			if recorder.Code != http.StatusBadRequest || recorder.Body.String() != "{\"detail\":\"invalid json\"}\n" {
				t.Fatalf("invalid JSON response mismatch: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}

	for name, call := range map[string]func(http.ResponseWriter, *http.Request){
		"list providers": h.ListProviders,
		"get provider":   h.GetProvider,
	} {
		t.Run(name+" permission", func(t *testing.T) {
			request := identityRouteRequest(http.MethodGet, "/v2/admin/identity-providers/missing", "", permission.Actor{})
			request.SetPathValue("provider_id", "missing")
			recorder := httptest.NewRecorder()
			call(recorder, request)
			if recorder.Code != http.StatusForbidden || recorder.Body.String() != "{\"detail\":\"permission denied\"}\n" {
				t.Fatalf("permission response mismatch: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestIdentityAuthorizationCancellationReturnsToIdentityManagementExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	h := identityapi.New(testutil.TestConfig(), db, redis, nil)
	owner := testutil.CreateUser(t, db, "identity-callback-owner@example.com", "Password123", "IdentityCallbackOwner", false)
	provider := model.IdentityProvider{
		ID: "identity-callback-provider", Name: "Callback Provider", IssuerURL: "https://callback.example",
		AuthorizationEndpoint: "https://callback.example/authorize", TokenEndpoint: "https://callback.example/token",
		JWKSURI: "https://callback.example/jwks", ClientID: "client", Scopes: []string{"openid"},
		Adapter: identitysvc.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(t.Context(), provider); err != nil {
		t.Fatal(err)
	}
	actor := identityRouteActor(t, db, owner.ID)

	req := identityRouteRequest(http.MethodPost, "/v2/identity-authorizations", `{"provider_id":"identity-callback-provider","intent":"link"}`, actor)
	rec := httptest.NewRecorder()
	h.StartAuthorization(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start authorization status=%d body=%q", rec.Code, rec.Body.String())
	}
	var started struct {
		AuthorizationURL string `json:"authorization_url"`
		ExpiresIn        int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil || authorizationURL.Query().Get("state") == "" || started.ExpiresIn != 600 {
		t.Fatalf("authorization start mismatch: result=%#v err=%v", started, err)
	}

	callbackTarget := "/v2/auth/oidc/callback?state=" + url.QueryEscape(authorizationURL.Query().Get("state")) + "&error=access_denied"
	req = identityRouteRequest(http.MethodGet, callbackTarget, "", permission.GuestActor())
	rec = httptest.NewRecorder()
	h.AuthorizationCallback(rec, req)
	wantLocation := "http://test/dashboard/identities?identity_error=authorization_incomplete"
	wantBody := `<a href="http://test/dashboard/identities?identity_error=authorization_incomplete">See Other</a>.` + "\n\n"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != wantLocation || rec.Body.String() != wantBody {
		t.Fatalf("cancel callback mismatch: status=%d location=%q body=%q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
}

func TestIdentityAuthorizationCallbackIssuesSessionAndRegistrationRedirectsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "identity-route-callback-key"
	var signedIDToken string
	var tokenCalls, jwksCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/token":
			tokenCalls++
			clientID, clientSecret, ok := req.BasicAuth()
			if !ok || clientID != "callback-client" || clientSecret != "callback-secret" {
				t.Fatalf("callback token auth id=%q secret=%q ok=%v", clientID, clientSecret, ok)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if req.Form.Get("grant_type") != "authorization_code" || req.Form.Get("code") == "" ||
				req.Form.Get("redirect_uri") != "http://localhost:8000/v2/auth/oidc/callback" ||
				req.Form.Get("code_verifier") == "" {
				t.Fatalf("callback token form=%#v", req.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":"callback-access","refresh_token":"callback-refresh","token_type":"Bearer","expires_in":3600,"scope":"openid email","id_token":%q}`, signedIDToken)
		case "/jwks":
			jwksCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":%q,"use":"sig","alg":"RS256","n":%q,"e":"AQAB"}]}`,
				keyID, base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	cfg := testutil.TestConfig()
	cache := testutil.NewMemoryRedis()
	h := identityapi.New(cfg, db, cache, nil)
	box, err := util.NewSecretBox(cfg.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := box.Encrypt("callback-secret")
	if err != nil {
		t.Fatal(err)
	}
	provider := model.IdentityProvider{
		ID: "identity-route-real-callback", Name: "Real Callback Provider", IssuerURL: server.URL,
		AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token",
		JWKSURI: server.URL + "/jwks", ClientID: "callback-client", ClientSecretCiphertext: encryptedSecret,
		Scopes: []string{"openid", "email"}, Adapter: identitysvc.AdapterGenericOIDC,
		Enabled: true, LoginEnabled: true, LinkEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	user := testutil.CreateUser(t, db, "identity-route-login@example.com", "Password123", "IdentityRouteLogin", false)
	external := model.ExternalIdentity{
		ID: "identity-route-login-external", UserID: user.ID, ProviderID: provider.ID,
		Subject: "existing-route-subject", Email: "old@remote.example", CreatedAt: 2, UpdatedAt: 2,
	}
	if err := db.Identities.CreateIdentity(ctx, external, model.ExternalIdentityCredential{IdentityID: external.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}

	startLogin := identityRouteRequest(http.MethodPost, "/v2/identity-authorizations",
		`{"provider_id":"identity-route-real-callback","intent":"login"}`, permission.GuestActor())
	loginStartRecorder := httptest.NewRecorder()
	h.StartAuthorization(loginStartRecorder, startLogin)
	loginAuthorizationURL := authorizationURLFromResponse(t, loginStartRecorder)
	signedIDToken = signRouteIdentityToken(t, privateKey, keyID, server.URL, "callback-client",
		loginAuthorizationURL.Query().Get("nonce"), external.Subject, "Existing Route User")
	loginState := loginAuthorizationURL.Query().Get("state")
	loginCallback := identityRouteRequest(http.MethodGet,
		"/v2/auth/oidc/callback?code=login-code&state="+url.QueryEscape(loginState), "", permission.GuestActor())
	loginRecorder := httptest.NewRecorder()
	h.AuthorizationCallback(loginRecorder, loginCallback)
	if loginRecorder.Code != http.StatusSeeOther || loginRecorder.Header().Get("Location") != "http://test/dashboard" ||
		loginRecorder.Body.String() != "<a href=\"http://test/dashboard\">See Other</a>.\n\n" {
		t.Fatalf("login callback status=%d location=%q body=%q", loginRecorder.Code, loginRecorder.Header().Get("Location"), loginRecorder.Body.String())
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range loginRecorder.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	if cookies["access_token"] == nil || cookies["access_token"].Value == "" || cookies["access_token"].HttpOnly != true ||
		cookies["refresh_token"] == nil || cookies["refresh_token"].Value == "" || cookies["refresh_token"].HttpOnly != true {
		t.Fatalf("login callback cookies=%#v", cookies)
	}
	updated, err := db.Identities.GetIdentity(ctx, external.ID)
	if err != nil || updated == nil || updated.DisplayName != "Existing Route User" || updated.LastLoginAt == nil {
		t.Fatalf("login callback identity=%#v err=%v", updated, err)
	}

	startRegistration := identityRouteRequest(http.MethodPost, "/v2/identity-authorizations",
		`{"provider_id":"identity-route-real-callback","intent":"login"}`, permission.GuestActor())
	registrationStartRecorder := httptest.NewRecorder()
	h.StartAuthorization(registrationStartRecorder, startRegistration)
	registrationAuthorizationURL := authorizationURLFromResponse(t, registrationStartRecorder)
	signedIDToken = signRouteIdentityToken(t, privateKey, keyID, server.URL, "callback-client",
		registrationAuthorizationURL.Query().Get("nonce"), "new-route-subject", "New Route User")
	registrationCallback := identityRouteRequest(http.MethodGet,
		"/v2/auth/oidc/callback?code=registration-code&state="+url.QueryEscape(registrationAuthorizationURL.Query().Get("state")),
		"", permission.GuestActor())
	registrationRecorder := httptest.NewRecorder()
	h.AuthorizationCallback(registrationRecorder, registrationCallback)
	location, err := url.Parse(registrationRecorder.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if registrationRecorder.Code != http.StatusSeeOther || location.Scheme != "http" || location.Host != "test" ||
		location.Path != "/register" || location.Query().Get("provider_id") != provider.ID ||
		location.Query().Get("identity_ticket") == "" || len(registrationRecorder.Result().Cookies()) != 0 {
		t.Fatalf("registration callback status=%d location=%q cookies=%#v", registrationRecorder.Code,
			registrationRecorder.Header().Get("Location"), registrationRecorder.Result().Cookies())
	}
	if tokenCalls != 2 || jwksCalls != 2 {
		t.Fatalf("OIDC callback calls token=%d jwks=%d want 2/2", tokenCalls, jwksCalls)
	}
}

func authorizationURLFromResponse(t *testing.T, recorder *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	var started struct {
		AuthorizationURL string `json:"authorization_url"`
		ExpiresIn        int    `json:"expires_in"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(started.AuthorizationURL)
	if err != nil || recorder.Code != http.StatusCreated || started.ExpiresIn != 600 ||
		parsed.Query().Get("state") == "" || parsed.Query().Get("nonce") == "" {
		t.Fatalf("authorization start status=%d result=%#v URL=%#v err=%v", recorder.Code, started, parsed, err)
	}
	return parsed
}

func signRouteIdentityToken(t *testing.T, privateKey *rsa.PrivateKey, keyID, issuer, audience, nonce, subject, name string) string {
	t.Helper()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": issuer, "sub": subject, "aud": audience, "nonce": nonce,
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(),
		"email": subject + "@remote.example", "email_verified": true, "name": name,
	})
	token.Header["kid"] = keyID
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func identityRouteRequest(method, target, body string, actor permission.Actor) *http.Request {
	var reader *strings.Reader
	reader = strings.NewReader(body)
	req := httptest.NewRequest(method, target, reader)
	return req.WithContext(shared.WithActor(req.Context(), actor))
}

func identityRouteActor(t *testing.T, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(t.Context(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return actor
}
