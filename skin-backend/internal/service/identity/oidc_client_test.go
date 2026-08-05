package identity_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/identity"
)

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
