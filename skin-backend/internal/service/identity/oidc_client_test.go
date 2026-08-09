package identity_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/identity"

	"github.com/golang-jwt/jwt/v5"
)

func TestStandardOIDCClientExchangesAndVerifiesSignedIdentityExactly(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "identity-test-key"
	var signedIDToken string
	var tokenCalls, jwksCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/token":
			tokenCalls++
			clientID, clientSecret, ok := req.BasicAuth()
			if !ok || clientID != "client-id" || clientSecret != "client-secret" {
				t.Fatalf("token basic auth id=%q secret=%q ok=%v", clientID, clientSecret, ok)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if req.Form.Get("grant_type") != "authorization_code" || req.Form.Get("code") != "authorization-code" ||
				req.Form.Get("redirect_uri") != "https://site.example/v2/auth/oidc/callback" ||
				req.Form.Get("code_verifier") != "pkce-verifier" {
				t.Fatalf("token request form=%#v", req.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":"access-token","refresh_token":"refresh-token","token_type":"Bearer","expires_in":3600,"scope":"openid email profile","id_token":%q}`, signedIDToken)
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

	now := time.Now()
	rawToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": server.URL, "sub": "remote-subject", "aud": "client-id",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(), "nonce": "expected-nonce",
		"email": "remote@example.com", "email_verified": true, "name": "Remote User",
		"preferred_username": "ignored-fallback", "picture": "https://images.example/avatar.png",
	})
	rawToken.Header["kid"] = keyID
	signedIDToken, err = rawToken.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	provider := model.IdentityProvider{
		IssuerURL: server.URL, AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token",
		JWKSURI: server.URL + "/jwks", ClientID: "client-id", Scopes: []string{"openid", "profile"},
	}
	before := time.Now()
	claims, tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).ExchangeAndVerify(
		context.Background(), provider, "client-secret", "authorization-code",
		"https://site.example/v2/auth/oidc/callback", "pkce-verifier", "expected-nonce",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantClaims := identity.OIDCClaims{
		Subject: "remote-subject", Email: "remote@example.com", EmailVerified: true,
		DisplayName: "Remote User", AvatarURL: "https://images.example/avatar.png",
	}
	if claims != wantClaims {
		t.Fatalf("verified claims=%#v want=%#v", claims, wantClaims)
	}
	if tokenCalls != 1 || jwksCalls != 1 || tokens.AccessToken != "access-token" || tokens.RefreshToken != "refresh-token" ||
		tokens.TokenType != "Bearer" || strings.Join(tokens.Scopes, " ") != "openid email profile" ||
		tokens.Expiry.Before(before.Add(59*time.Minute)) || tokens.Expiry.After(before.Add(61*time.Minute)) {
		t.Fatalf("verified tokens=%#v token_calls=%d jwks_calls=%d", tokens, tokenCalls, jwksCalls)
	}
}

func TestStandardOIDCClientRefreshUsesProviderTokenEndpointExactly(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != http.MethodPost || req.URL.Path != "/token" {
			t.Fatalf("refresh request=%s %s", req.Method, req.URL.Path)
		}
		clientID, clientSecret, ok := req.BasicAuth()
		if !ok || clientID != "client-id" || clientSecret != "client-secret" {
			t.Fatalf("refresh basic auth id=%q secret=%q ok=%v", clientID, clientSecret, ok)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("grant_type") != "refresh_token" || req.Form.Get("refresh_token") != "old-refresh" || len(req.Form) != 2 {
			t.Fatalf("refresh form=%#v", req.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600,"scope":"openid email"}`))
	}))
	defer server.Close()
	provider := model.IdentityProvider{ClientID: "client-id", TokenEndpoint: server.URL + "/token"}
	before := time.Now()
	tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).Refresh(context.Background(), provider, "client-secret", "old-refresh", []string{"openid", "profile"})
	if err != nil || calls != 1 || tokens.AccessToken != "new-access" || tokens.RefreshToken != "new-refresh" || tokens.TokenType != "Bearer" || strings.Join(tokens.Scopes, " ") != "openid email" || tokens.Expiry.Before(before.Add(59*time.Minute)) || tokens.Expiry.After(before.Add(61*time.Minute)) {
		t.Fatalf("refreshed tokens=%#v calls=%d err=%v", tokens, calls, err)
	}
}

func TestStandardOIDCClientClassifiesRejectedRefreshExactly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
	}))
	defer server.Close()
	_, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).Refresh(
		context.Background(), model.IdentityProvider{ClientID: "client", TokenEndpoint: server.URL},
		"secret", "rejected-refresh", []string{"openid"},
	)
	if !errors.Is(err, identity.ErrRefreshRejected) {
		t.Fatalf("rejected refresh error=%#v; want ErrRefreshRejected", err)
	}
}

func TestStandardOIDCClientRejectsMalformedExchangeAndRefreshResponsesExactly(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantDetail string
	}{
		{name: "exchange rejected", status: http.StatusBadRequest, body: `{"error":"invalid_grant"}`, wantDetail: "OIDC token exchange failed"},
		{name: "missing ID token", status: http.StatusOK, body: `{"access_token":"access","token_type":"Bearer"}`, wantDetail: "OIDC token response did not include an ID token"},
		{name: "invalid ID token", status: http.StatusOK, body: `{"access_token":"access","token_type":"Bearer","id_token":"not-a-jwt"}`, wantDetail: "OIDC ID token verification failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/token" {
					http.NotFound(w, req)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			provider := model.IdentityProvider{
				IssuerURL: server.URL, AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token",
				JWKSURI: server.URL + "/jwks", ClientID: "client", Scopes: []string{"openid"},
			}
			claims, tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).ExchangeAndVerify(
				context.Background(), provider, "secret", "code", "https://site.example/callback", "verifier", "nonce",
			)
			if claims != (identity.OIDCClaims{}) || tokens.AccessToken != "" || err == nil || err.Error() != tc.wantDetail {
				t.Fatalf("malformed exchange claims=%#v tokens=%#v err=%v", claims, tokens, err)
			}
		})
	}

	t.Run("refresh upstream failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
		}))
		defer server.Close()
		tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).Refresh(
			context.Background(), model.IdentityProvider{ClientID: "client", TokenEndpoint: server.URL},
			"secret", "refresh", []string{"openid"},
		)
		if tokens.AccessToken != "" || err == nil || err.Error() != "OIDC token refresh failed" {
			t.Fatalf("upstream refresh tokens=%#v err=%v", tokens, err)
		}
	})

	t.Run("refresh missing access token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"   ","refresh_token":"rotated","token_type":"Bearer"}`))
		}))
		defer server.Close()
		tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).Refresh(
			context.Background(), model.IdentityProvider{ClientID: "client", TokenEndpoint: server.URL},
			"secret", "refresh", []string{"openid"},
		)
		if tokens.AccessToken != "" || err == nil || err.Error() != "OIDC token refresh response did not include an access token" {
			t.Fatalf("missing access refresh tokens=%#v err=%v", tokens, err)
		}
	})

	t.Run("refresh retains requested scopes when response omits scope", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fallback-access","token_type":"Bearer","expires_in":300}`))
		}))
		defer server.Close()
		tokens, err := (identity.StandardOIDCClient{HTTPClient: server.Client()}).Refresh(
			context.Background(), model.IdentityProvider{ClientID: "client", TokenEndpoint: server.URL},
			"secret", "refresh", []string{"openid", "profile"},
		)
		if err != nil || tokens.AccessToken != "fallback-access" || !reflect.DeepEqual(tokens.Scopes, []string{"openid", "profile"}) {
			t.Fatalf("fallback scope refresh tokens=%#v err=%v", tokens, err)
		}
	})
}
