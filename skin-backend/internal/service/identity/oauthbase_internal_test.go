package identity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestExchangeAuthorizationCodeSendsQueryStringAndParsesJSONExactly(t *testing.T) {
	var gotMethod, gotUserAgent, gotAccept, gotContentType string
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotUserAgent = req.Header.Get("User-Agent")
		gotAccept = req.Header.Get("Accept")
		gotContentType = req.Header.Get("Content-Type")
		gotQuery = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"json-access","refresh_token":"json-refresh","token_type":"Bearer","expires_in":3600,"openid":"open-1"}`))
	}))
	defer server.Close()

	result, err := exchangeAuthorizationCode(context.Background(), server.Client(), server.URL+"/token", http.MethodGet, url.Values{
		"grant_type": {oauthGrantCode}, "code": {"the-code"}, "fmt": {"json"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotUserAgent != oauthUserAgent || gotAccept != "application/json" || gotContentType != "" {
		t.Fatalf("GET exchange headers method=%q ua=%q accept=%q content_type=%q", gotMethod, gotUserAgent, gotAccept, gotContentType)
	}
	wantQuery := url.Values{"grant_type": {"authorization_code"}, "code": {"the-code"}, "fmt": {"json"}}
	if gotQuery.Encode() != wantQuery.Encode() {
		t.Fatalf("GET exchange query=%#v", gotQuery)
	}
	if result.AccessToken != "json-access" || result.RefreshToken != "json-refresh" || result.TokenType != "Bearer" ||
		result.ExpiresIn != 3600 || result.Fields["openid"] != "open-1" {
		t.Fatalf("JSON exchange result=%#v", result)
	}
}

func TestExchangeAuthorizationCodePostsFormBodyAndParsesFormEncodedResponseExactly(t *testing.T) {
	var gotMethod, gotContentType string
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotContentType = req.Header.Get("Content-Type")
		rawBody, _ = io.ReadAll(req.Body)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("access_token=form-access&expires_in=7776000&openid=open-form"))
	}))
	defer server.Close()

	params := url.Values{"grant_type": {oauthGrantCode}, "client_secret": {"shared-secret"}}
	result, err := exchangeAuthorizationCode(context.Background(), server.Client(), server.URL+"/token", http.MethodPost, params)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("POST exchange request method=%q content_type=%q", gotMethod, gotContentType)
	}
	if got, _ := url.ParseQuery(string(rawBody)); got.Encode() != params.Encode() {
		t.Fatalf("POST exchange body=%q want=%q", string(rawBody), params.Encode())
	}
	if req, _ := url.Parse(server.URL); req.RawQuery != "" {
		t.Fatalf("POST exchange must not carry query parameters: %q", req.RawQuery)
	}
	if result.AccessToken != "form-access" || result.ExpiresIn != 7776000 || result.Fields["openid"] != "open-form" ||
		result.RefreshToken != "" || result.TokenType != "" {
		t.Fatalf("form exchange result=%#v", result)
	}
}

func TestExchangeAuthorizationCodeClassifiesFailuresWithoutMaskingUpstreamRejections(t *testing.T) {
	t.Run("upstream rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `callback( {"error":100010} );`, http.StatusBadRequest)
		}))
		defer server.Close()
		_, err := exchangeAuthorizationCode(context.Background(), server.Client(), server.URL+"/token", http.MethodGet, url.Values{})
		var upstream oauthUpstreamError
		if !errors.As(err, &upstream) || upstream.Status != http.StatusBadRequest || !strings.Contains(upstream.Body, "100010") {
			t.Fatalf("rejection error=%#v want oauthUpstreamError(400)", err)
		}
		classified := classifyOAuthExchangeError(err)
		if classified.Reason != "denied" || classified.Object != "identity_token" || classified.Operation != "exchange" {
			t.Fatalf("classified rejection=%#v", classified)
		}
	})

	t.Run("malformed payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("%zz&&"))
		}))
		defer server.Close()
		_, err := exchangeAuthorizationCode(context.Background(), server.Client(), server.URL+"/token", http.MethodGet, url.Values{})
		if err == nil || !strings.Contains(err.Error(), "neither JSON nor form-encoded") {
			t.Fatalf("malformed payload error=%v", err)
		}
		if classified := classifyOAuthExchangeError(err); classified.Reason != "failed" {
			t.Fatalf("classified malformed reason=%q", classified.Reason)
		}
	})

	t.Run("transport failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		server.Close()
		_, err := exchangeAuthorizationCode(context.Background(), server.Client(), server.URL+"/token", http.MethodGet, url.Values{})
		if err == nil || errors.Is(err, oauthUpstreamError{}) {
			t.Fatalf("transport failure error=%v", err)
		}
		if classified := classifyOAuthExchangeError(err); classified.Reason != "failed" {
			t.Fatalf("classified transport reason=%q", classified.Reason)
		}
	})
}

func TestFetchOAuthProfileJSONAttachesBearerMergesQueryAndRejectsBadPayloads(t *testing.T) {
	t.Run("bearer and query", func(t *testing.T) {
		var gotAuthorization string
		var gotQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			gotAuthorization = req.Header.Get("Authorization")
			gotQuery = req.URL.Query()
			_, _ = w.Write([]byte(`{"ret":0,"nickname":"Nick"}`))
		}))
		defer server.Close()
		profile, err := fetchOAuthProfileJSON(context.Background(), server.Client(), server.URL+"/user", url.Values{
			"access_token": {"at"}, "openid": {"open-1"},
		}, "bearer-at")
		if err != nil {
			t.Fatal(err)
		}
		if gotAuthorization != "Bearer bearer-at" || gotQuery.Get("access_token") != "at" || gotQuery.Get("openid") != "open-1" {
			t.Fatalf("profile request authorization=%q query=%#v", gotAuthorization, gotQuery)
		}
		if profile["nickname"] != "Nick" || profile["ret"] != float64(0) {
			t.Fatalf("profile=%#v", profile)
		}
	})

	t.Run("no bearer header when token empty", func(t *testing.T) {
		var gotAuthorization string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			gotAuthorization = req.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()
		if _, err := fetchOAuthProfileJSON(context.Background(), server.Client(), server.URL+"/user", url.Values{}, "   "); err != nil {
			t.Fatal(err)
		}
		if gotAuthorization != "" {
			t.Fatalf("unexpected authorization header %q", gotAuthorization)
		}
	})

	t.Run("upstream rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "expired", http.StatusUnauthorized)
		}))
		defer server.Close()
		_, err := fetchOAuthProfileJSON(context.Background(), server.Client(), server.URL+"/user", url.Values{}, "")
		var upstream oauthUpstreamError
		if !errors.As(err, &upstream) || upstream.Status != http.StatusUnauthorized {
			t.Fatalf("profile rejection error=%#v", err)
		}
	})

	t.Run("invalid payloads", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			switch req.URL.Query().Get("case") {
			case "array":
				_, _ = w.Write([]byte(`[1,2]`))
			case "null":
				_, _ = w.Write([]byte(`null`))
			default:
				_, _ = w.Write([]byte(`not-json`))
			}
		}))
		defer server.Close()
		for _, tc := range []struct{ name, payloadCase string }{
			{name: "non object array", payloadCase: "array"},
			{name: "json null", payloadCase: "null"},
			{name: "not json", payloadCase: ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				endpoint := server.URL + "/user"
				if tc.payloadCase != "" {
					endpoint += "?case=" + tc.payloadCase
				}
				if _, err := fetchOAuthProfileJSON(context.Background(), server.Client(), endpoint, url.Values{}, ""); err == nil {
					t.Fatalf("%s should fail", tc.name)
				}
			})
		}
	})
}

func TestParseOAuthTokenResponseNormalizesScalarTypesOnly(t *testing.T) {
	result, err := parseOAuthTokenResponse([]byte(`{"access_token":"at","expires_in":86400,"is_vip":true,"nested":{"ignored":1},"list":[1],"missing":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "at" || result.ExpiresIn != 86400 || result.Fields["is_vip"] != "true" {
		t.Fatalf("parsed result=%#v", result)
	}
	if _, exists := result.Fields["nested"]; exists {
		t.Fatalf("object fields must not leak into scalar map: %#v", result.Fields)
	}
	if _, exists := result.Fields["list"]; exists {
		t.Fatalf("array fields must not leak into scalar map: %#v", result.Fields)
	}
	if _, exists := result.Fields["missing"]; exists {
		t.Fatalf("null fields must not leak into scalar map: %#v", result.Fields)
	}
}
