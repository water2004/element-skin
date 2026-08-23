package oauth_test

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"element-skin/backend/internal/testutil"

	"github.com/golang-jwt/jwt/v5"
)

func TestOpenIDAuthorizationCodeFlowIssuesVerifiablePairwiseIdentityExactly(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oidc-server-user@example.com", "Password123", "OIDC Server User", false)
	session := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "OIDC relying party",
		"redirect_uri": "https://relying-party.example/callback",
		"website_url":  "https://relying-party.example",
		"client_type":  "public",
		"permissions":  []string{},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create OIDC client status=%d body=%q", createRes.Code, createRes.Body.String())
	}
	client := decodeMap(t, createRes.Body.Bytes())
	clientID := client["client_id"].(string)
	if clientID == "" || client["client_secret"] != nil || len(client["permissions"].([]any)) != 0 {
		t.Fatalf("OIDC-only public client response mismatch: %#v", client)
	}
	activateOAuthClient(t, db, clientID)

	metadataRec := httptest.NewRecorder()
	router.ServeHTTP(metadataRec, httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil))
	metadata := decodeMap(t, metadataRec.Body.Bytes())
	if metadataRec.Code != http.StatusOK || metadata["issuer"] != cfg.APIURL ||
		metadata["authorization_endpoint"] != cfg.SiteURL+"/oauth/authorize" ||
		metadata["token_endpoint"] != cfg.APIURL+"/oauth/token" ||
		metadata["userinfo_endpoint"] != cfg.APIURL+"/oauth/userinfo" ||
		metadata["jwks_uri"] != cfg.APIURL+"/oauth/jwks" {
		t.Fatalf("OpenID configuration mismatch: status=%d metadata=%#v", metadataRec.Code, metadata)
	}
	if values := stringSet(metadata["subject_types_supported"].([]any)); len(values) != 1 || !values["pairwise"] {
		t.Fatalf("subject types mismatch: %#v", metadata["subject_types_supported"])
	}
	if values := stringSet(metadata["id_token_signing_alg_values_supported"].([]any)); len(values) != 1 || !values["RS256"] {
		t.Fatalf("ID-token algorithms mismatch: %#v", metadata["id_token_signing_alg_values_supported"])
	}

	onlineVerifier := "oidc-online-verifier-abcdefghijklmnopqrstuvwxyz"
	onlineAuthorization := doJSON(t, router, http.MethodPost, "/oauth/authorize", map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          "https://relying-party.example/callback",
		"scope":                 "openid profile",
		"state":                 "online-state",
		"nonce":                 "online-nonce",
		"code_challenge":        pkceChallenge(onlineVerifier),
		"code_challenge_method": "S256",
	}, session, "")
	if onlineAuthorization.Code != http.StatusOK {
		t.Fatalf("online OIDC authorization status=%d body=%q", onlineAuthorization.Code, onlineAuthorization.Body.String())
	}
	onlineCode := decodeMap(t, onlineAuthorization.Body.Bytes())["code"].(string)
	onlineTokenRec := doForm(t, router, "/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {onlineCode},
		"code_verifier": {onlineVerifier},
		"redirect_uri":  {"https://relying-party.example/callback"},
	}, "", "")
	onlineToken := decodeMap(t, onlineTokenRec.Body.Bytes())
	if onlineTokenRec.Code != http.StatusOK || onlineToken["access_token"] == "" || onlineToken["id_token"] == "" ||
		onlineToken["refresh_token"] != nil || onlineToken["scope"] != "openid profile" {
		t.Fatalf("online OIDC token response mismatch: status=%d token=%#v", onlineTokenRec.Code, onlineToken)
	}
	var onlineRefreshCount int
	if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE client_id=$1`, clientID).Scan(&onlineRefreshCount); err != nil {
		t.Fatal(err)
	}
	if onlineRefreshCount != 0 {
		t.Fatalf("online OIDC authorization stored %d refresh tokens, want 0", onlineRefreshCount)
	}

	verifier := "oidc-server-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	authorizeQuery := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"https://relying-party.example/callback"},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {"oidc-state"},
		"nonce":                 {"oidc-nonce"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	infoReq := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+authorizeQuery.Encode(), nil)
	infoReq.AddCookie(session)
	infoRec := httptest.NewRecorder()
	router.ServeHTTP(infoRec, infoReq)
	info := decodeMap(t, infoRec.Body.Bytes())
	if infoRec.Code != http.StatusOK || len(info["scopes"].([]any)) != 0 {
		t.Fatalf("OIDC authorization details mismatch: status=%d body=%#v", infoRec.Code, info)
	}
	if scopes := info["oidc_scopes"].([]any); len(scopes) != 4 || scopes[0] != "email" || scopes[1] != "offline_access" || scopes[2] != "openid" || scopes[3] != "profile" {
		t.Fatalf("OIDC scope details mismatch: %#v", info["oidc_scopes"])
	}

	authorizeRes := doJSON(t, router, http.MethodPost, "/oauth/authorize", map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          "https://relying-party.example/callback",
		"scope":                 "openid profile email offline_access",
		"state":                 "oidc-state",
		"nonce":                 "oidc-nonce",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}, session, "")
	if authorizeRes.Code != http.StatusOK {
		t.Fatalf("approve OIDC authorization status=%d body=%q", authorizeRes.Code, authorizeRes.Body.String())
	}
	authorization := decodeMap(t, authorizeRes.Body.Bytes())
	code := authorization["code"].(string)
	if code == "" || authorization["state"] != "oidc-state" ||
		authorization["redirect_url"] != "https://relying-party.example/callback?code="+url.QueryEscape(code)+"&state=oidc-state" {
		t.Fatalf("OIDC authorization response mismatch: %#v", authorization)
	}

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://relying-party.example/callback"},
	}
	tokenRec := doForm(t, router, "/oauth/token", tokenForm, "", "")
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("OIDC token status=%d body=%q", tokenRec.Code, tokenRec.Body.String())
	}
	token := decodeMap(t, tokenRec.Body.Bytes())
	accessToken, _ := token["access_token"].(string)
	refreshToken, _ := token["refresh_token"].(string)
	idToken, _ := token["id_token"].(string)
	if accessToken == "" || refreshToken == "" || idToken == "" || token["token_type"] != "Bearer" ||
		token["scope"] != "email offline_access openid profile" || len(token["permissions"].([]any)) != 0 {
		t.Fatalf("OIDC token response mismatch: %#v", token)
	}

	publicKey, keyID := oidcPublicKey(t, router)
	claims := parseAndVerifyIDToken(t, idToken, publicKey, keyID, clientID)
	subject, _ := claims["sub"].(string)
	if subject == "" || subject == user.ID || claims["iss"] != cfg.APIURL || claims["nonce"] != "oidc-nonce" ||
		claims["name"] != "OIDC Server User" || claims["preferred_username"] != "OIDC Server User" ||
		claims["email"] != "oidc-server-user@example.com" || claims["email_verified"] != false {
		t.Fatalf("ID-token claims mismatch: %#v", claims)
	}

	userinfo := doJSON(t, router, http.MethodGet, "/oauth/userinfo", nil, nil, accessToken)
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status=%d body=%q", userinfo.Code, userinfo.Body.String())
	}
	userinfoClaims := decodeMap(t, userinfo.Body.Bytes())
	if userinfoClaims["sub"] != subject || userinfoClaims["name"] != "OIDC Server User" ||
		userinfoClaims["email"] != "oidc-server-user@example.com" || userinfoClaims["locale"] != "zh_CN" ||
		len(userinfoClaims) != 6 {
		t.Fatalf("userinfo claims mismatch: %#v", userinfoClaims)
	}

	me := doJSON(t, router, http.MethodGet, "/v2/users/me", nil, nil, accessToken)
	if me.Code != http.StatusUnauthorized || me.Body.String() != "{\"error\":{\"object\":\"authentication\",\"operation\":\"verify\",\"reason\":\"required\"}}\n" {
		t.Fatalf("OIDC-only access token must not authorize site API: status=%d body=%q", me.Code, me.Body.String())
	}

	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	refreshRec := doForm(t, router, "/oauth/token", refreshForm, "", "")
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("OIDC refresh status=%d body=%q", refreshRec.Code, refreshRec.Body.String())
	}
	refreshed := decodeMap(t, refreshRec.Body.Bytes())
	refreshClaims := parseAndVerifyIDToken(t, refreshed["id_token"].(string), publicKey, keyID, clientID)
	if refreshClaims["sub"] != subject || refreshClaims["nonce"] != nil || refreshed["scope"] != "email offline_access openid profile" {
		t.Fatalf("refreshed ID token mismatch: token=%#v claims=%#v", refreshed, refreshClaims)
	}

	grants, err := db.OAuth.ListGrantsByUser(t.Context(), user.ID, 10)
	if err != nil || len(grants) != 1 || len(grants[0].OIDCScopes) != 4 {
		t.Fatalf("stored OIDC grant mismatch: grants=%#v err=%v", grants, err)
	}
	revoke := doJSON(t, router, http.MethodDelete, "/v2/oauth/grants/"+grants[0].ID, nil, session, "")
	if revoke.Code != http.StatusNoContent || revoke.Body.Len() != 0 {
		t.Fatalf("revoke OIDC grant status=%d body=%q", revoke.Code, revoke.Body.String())
	}
	invalidUserInfo := doJSON(t, router, http.MethodGet, "/oauth/userinfo", nil, nil, refreshed["access_token"].(string))
	if invalidUserInfo.Code != http.StatusUnauthorized || invalidUserInfo.Header().Get("WWW-Authenticate") != `Bearer error="invalid_token"` ||
		invalidUserInfo.Body.String() != "{\"error\":\"invalid_token\"}\n" {
		t.Fatalf("revoked grant userinfo mismatch: status=%d auth=%q body=%q", invalidUserInfo.Code, invalidUserInfo.Header().Get("WWW-Authenticate"), invalidUserInfo.Body.String())
	}
}

func TestOpenIDAuthorizationRejectsScopesWithoutOpenIDExactly(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oidc-scope-error@example.com", "Password123", "OIDCScopeError", false)
	client := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name": "OIDC invalid scope", "redirect_uri": "https://invalid-scope.example/callback",
		"client_type": "public", "permissions": []string{},
	}, webCookie(t, cfg.JWTSecret, user.ID), "")
	if client.Code != http.StatusCreated {
		t.Fatalf("create invalid-scope client status=%d body=%q", client.Code, client.Body.String())
	}
	clientID := decodeMap(t, client.Body.Bytes())["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	query := url.Values{
		"response_type": {"code"}, "client_id": {clientID},
		"redirect_uri": {"https://invalid-scope.example/callback"}, "scope": {"profile email"},
		"code_challenge":        {pkceChallenge("invalid-oidc-scope-verifier-abcdefghijklmnopqrstuvwxyz")},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil)
	req.AddCookie(webCookie(t, cfg.JWTSecret, user.ID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"oidc_scope\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
		t.Fatalf("OIDC scope validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func oidcPublicKey(t *testing.T, router http.Handler) (*rsa.PublicKey, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/jwks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("JWKS status=%d body=%q", rec.Code, rec.Body.String())
	}
	keys := decodeMap(t, rec.Body.Bytes())["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("JWKS key count mismatch: %#v", keys)
	}
	key := keys[0].(map[string]any)
	if key["kty"] != "RSA" || key["use"] != "sig" || key["alg"] != "RS256" || key["kid"] == "" {
		t.Fatalf("JWKS key metadata mismatch: %#v", key)
	}
	modulus, err := base64.RawURLEncoding.DecodeString(key["n"].(string))
	if err != nil {
		t.Fatal(err)
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(key["e"].(string))
	if err != nil {
		t.Fatal(err)
	}
	paddedExponent := make([]byte, 4)
	copy(paddedExponent[4-len(exponentBytes):], exponentBytes)
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(binary.BigEndian.Uint32(paddedExponent))}, key["kid"].(string)
}

func parseAndVerifyIDToken(t *testing.T, raw string, publicKey *rsa.PublicKey, keyID, audience string) map[string]any {
	t.Helper()
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 || token.Header["kid"] != keyID {
			t.Fatalf("ID-token header mismatch: %#v", token.Header)
		}
		return publicKey, nil
	}, jwt.WithAudience(audience), jwt.WithIssuer(testutil.TestConfig().APIURL), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !token.Valid {
		t.Fatalf("ID-token verification failed: valid=%v err=%v token=%q", token != nil && token.Valid, err, raw)
	}
	claims := map[string]any(token.Claims.(jwt.MapClaims))
	if _, ok := claims["exp"].(float64); !ok {
		t.Fatalf("ID token exp claim mismatch: %#v", claims)
	}
	if _, ok := claims["iat"].(float64); !ok {
		t.Fatalf("ID token iat claim mismatch: %#v", claims)
	}
	return claims
}
