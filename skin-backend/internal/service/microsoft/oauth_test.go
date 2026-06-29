package microsoft_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

func TestMicrosoftHTTPClientExchangeCodeRequestShape(t *testing.T) {
	var gotMethod, gotContentType, gotURL string
	var gotForm map[string]string
	client := microsoft.MicrosoftHTTPClient{
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotMethod = req.Method
			gotURL = req.URL.String()
			gotContentType = req.Header.Get("Content-Type")
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatal(err)
			}
			gotForm = map[string]string{
				"client_id":     form.Get("client_id"),
				"client_secret": form.Get("client_secret"),
				"code":          form.Get("code"),
				"redirect_uri":  form.Get("redirect_uri"),
				"grant_type":    form.Get("grant_type"),
			}
			return jsonResponse(`{"access_token":"ms_access"}`), nil
		})},
		ClientID:     "client_id",
		ClientSecret: "secret",
		RedirectURI:  "https://skin.example/v1/imports/microsoft/callback",
	}
	out, err := client.ExchangeCodeForToken(context.Background(), "auth_code")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "https://login.microsoftonline.com/consumers/oauth2/v2.0/token" || gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("unexpected token request shape: method=%s url=%s content-type=%s", gotMethod, gotURL, gotContentType)
	}
	if gotForm["client_id"] != "client_id" || gotForm["client_secret"] != "secret" || gotForm["code"] != "auth_code" ||
		gotForm["redirect_uri"] != "https://skin.example/v1/imports/microsoft/callback" || gotForm["grant_type"] != "authorization_code" {
		t.Fatalf("unexpected token request form: %#v", gotForm)
	}
	if out["access_token"] != "ms_access" {
		t.Fatalf("response should decode JSON exactly: %#v", out)
	}
}
