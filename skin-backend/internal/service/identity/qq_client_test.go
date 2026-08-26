package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/identity"
)

func TestQQClientExchangesCodeFetchesProfileAndReturnsNoPersistableTokens(t *testing.T) {
	var tokenCalls, profileCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("User-Agent") == "" {
			t.Fatal("upstream requests must carry a User-Agent header")
		}
		switch req.URL.Path {
		case "/oauth2.0/token":
			tokenCalls++
			if req.Method != http.MethodGet || len(req.URL.Query()) != 7 {
				t.Fatalf("token request method=%s query=%#v", req.Method, req.URL.Query())
			}
			query := req.URL.Query()
			if query.Get("grant_type") != "authorization_code" || query.Get("client_id") != "app-id" ||
				query.Get("client_secret") != "app-key" || query.Get("code") != "auth-code" ||
				query.Get("redirect_uri") != "https://site.example/v2/auth/oidc/callback" ||
				query.Get("fmt") != "json" || query.Get("need_openid") != "1" {
				t.Fatalf("token request query=%#v", query)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"QQ-ACCESS","expires_in":5184000,"refresh_token":"QQ-ROTATING","openid":"OPENID-1"}`))
		case "/user/get_user_info":
			profileCalls++
			query := req.URL.Query()
			if query.Get("access_token") != "QQ-ACCESS" || query.Get("oauth_consumer_key") != "app-id" ||
				query.Get("openid") != "OPENID-1" || query.Get("format") != "json" {
				t.Fatalf("profile request query=%#v", query)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ret":0,"msg":"","nickname":"QQ昵称","figureurl_qq_2":"https://qlogo.example/100.png","figureurl_qq_1":"https://qlogo.example/40.png","gender":"男"}`))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	provider := model.IdentityProvider{
		IssuerURL:        identity.QQUIssuerURL,
		TokenEndpoint:    server.URL + "/oauth2.0/token",
		UserInfoEndpoint: server.URL + "/user/get_user_info",
		ClientID:         "app-id",
		Scopes:           []string{"get_user_info"},
		Adapter:          identity.AdapterQQ,
	}
	claims, tokens, err := (identity.QQClient{HTTPClient: server.Client()}).ExchangeAndVerify(
		context.Background(), provider, "app-key", "auth-code",
		"https://site.example/v2/auth/oidc/callback", "ignored-verifier", "ignored-nonce",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantClaims := identity.OIDCClaims{
		Subject: "OPENID-1", DisplayName: "QQ昵称", AvatarURL: "https://qlogo.example/100.png",
	}
	if claims != wantClaims {
		t.Fatalf("qq claims=%#v want=%#v", claims, wantClaims)
	}
	wantTokens := identity.OIDCTokens{Scopes: []string{"get_user_info"}}
	if tokens.AccessToken != "" || tokens.RefreshToken != "" || tokens.TokenType != "" || !tokens.Expiry.IsZero() ||
		!reflect.DeepEqual(tokens.Scopes, wantTokens.Scopes) {
		t.Fatalf("qq tokens must stay unpersistable: %#v want %#v", tokens, wantTokens)
	}
	if tokenCalls != 1 || profileCalls != 1 {
		t.Fatalf("upstream calls token=%d profile=%d", tokenCalls, profileCalls)
	}
}

func TestQQClientFallsBackToTheGuaranteedSmallAvatarWhenLargeOneIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/oauth2.0/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","openid":"OPENID-FALLBACK"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ret":0,"msg":"","nickname":"SmallAvatar","figureurl_qq_1":"https://qlogo.example/only-40.png"}`))
	}))
	defer server.Close()

	provider := model.IdentityProvider{
		TokenEndpoint:    server.URL + "/oauth2.0/token",
		UserInfoEndpoint: server.URL + "/user/get_user_info",
		ClientID:         "app-id",
		Scopes:           []string{"get_user_info"},
	}
	claims, _, err := (identity.QQClient{HTTPClient: server.Client()}).ExchangeAndVerify(
		context.Background(), provider, "app-key", "code", "https://site.example/callback", "", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := identity.OIDCClaims{Subject: "OPENID-FALLBACK", DisplayName: "SmallAvatar", AvatarURL: "https://qlogo.example/only-40.png"}
	if claims != want {
		t.Fatalf("fallback claims=%#v want=%#v", claims, want)
	}
}

func TestQQClientClassifiesIncompleteDeniedAndFailedUpstreamsExactly(t *testing.T) {
	tests := []struct {
		name               string
		tokenStatus        int
		tokenBody          string
		profileBody        string
		closeServer        bool
		wantClassification string
	}{
		{
			name:               "missing openid",
			tokenBody:          `{"access_token":"AT","expires_in":3600}`,
			wantClassification: "identity_token.exchange.incomplete",
		},
		{
			name:               "token endpoint rejects the code",
			tokenStatus:        http.StatusBadRequest,
			tokenBody:          `callback( {"error":100010,"error_description":"bad code"} );`,
			wantClassification: "identity_token.exchange.denied",
		},
		{
			name:               "token endpoint reports an error with HTTP 200 like the official SDK",
			tokenBody:          `{"error":100016,"error_description":"access_token is illegal"}`,
			wantClassification: "identity_token.exchange.denied",
		},
		{
			name:               "get_user_info refuses the session",
			tokenBody:          `{"access_token":"EXPIRED","openid":"OPENID-DENIED"}`,
			profileBody:        `{"ret":1002,"msg":"请先登录"}`,
			wantClassification: "identity_token.exchange.denied",
		},
		{
			name:               "transport failure",
			tokenBody:          `{"access_token":"AT","openid":"NEVER-CALLED"}`,
			closeServer:        true,
			wantClassification: "identity_token.exchange.failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path == "/oauth2.0/token" {
					if tc.tokenStatus != 0 {
						w.WriteHeader(tc.tokenStatus)
					}
					_, _ = w.Write([]byte(tc.tokenBody))
					return
				}
				_, _ = w.Write([]byte(tc.profileBody))
			}))
			client := server.Client()
			if tc.closeServer {
				server.Close()
			} else {
				defer server.Close()
			}
			provider := model.IdentityProvider{
				TokenEndpoint:    server.URL + "/oauth2.0/token",
				UserInfoEndpoint: server.URL + "/user/get_user_info",
				ClientID:         "app-id",
				Scopes:           []string{"get_user_info"},
			}
			claims, tokens, err := (identity.QQClient{HTTPClient: client}).ExchangeAndVerify(
				context.Background(), provider, "app-key", "code", "https://site.example/callback", "", "",
			)
			if claims != (identity.OIDCClaims{}) || tokens.Scopes != nil || err == nil || err.Error() != tc.wantClassification {
				t.Fatalf("qq exchange claims=%#v tokens=%#v err=%v want=%s", claims, tokens, err, tc.wantClassification)
			}
		})
	}
}
