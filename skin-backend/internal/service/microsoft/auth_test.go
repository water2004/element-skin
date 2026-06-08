package microsoft_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

type fakeMicrosoftClient struct {
	calls []string
}

func (f *fakeMicrosoftClient) ExchangeCodeForToken(context.Context, string) (map[string]any, error) {
	f.calls = append(f.calls, "exchange")
	return map[string]any{"access_token": "ms_access_token"}, nil
}

func (f *fakeMicrosoftClient) AuthenticateXBL(context.Context, string) (string, string, error) {
	f.calls = append(f.calls, "xbl")
	return "xbl_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateXSTS(context.Context, string) (string, string, error) {
	f.calls = append(f.calls, "xsts")
	return "xsts_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateMinecraft(context.Context, string, string) (string, error) {
	f.calls = append(f.calls, "minecraft")
	return "mc_access_token", nil
}

func (f *fakeMicrosoftClient) CheckGameOwnership(context.Context, string) (bool, error) {
	f.calls = append(f.calls, "ownership")
	return true, nil
}

func (f *fakeMicrosoftClient) GetMinecraftProfile(context.Context, string) (map[string]any, error) {
	f.calls = append(f.calls, "profile")
	return map[string]any{"id": "uuid", "name": "McPlayer"}, nil
}

type missingAccessMicrosoftClient struct {
	fakeMicrosoftClient
}

func (m *missingAccessMicrosoftClient) ExchangeCodeForToken(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMicrosoftAuthorizationURL(t *testing.T) {
	u := microsoft.MicrosoftAuthorizationURL("client_id", "https://redirect.com", "state123")
	if !strings.Contains(u, "client_id=client_id") || !strings.Contains(u, "state=state123") || !strings.Contains(u, "redirect_uri=https%3A%2F%2Fredirect.com") {
		t.Fatalf("unexpected authorization URL: %s", u)
	}
}

func TestMicrosoftAuthFlowComplete(t *testing.T) {
	client := &fakeMicrosoftClient{}
	res, err := (microsoft.MicrosoftAuthFlow{Client: client}).Complete(context.Background(), "auth_code")
	if err != nil {
		t.Fatal(err)
	}
	if res["mc_access_token"] != "mc_access_token" || res["has_game"] != true {
		t.Fatalf("unexpected auth flow result: %#v", res)
	}
	profile := res["profile"].(map[string]any)
	if profile["name"] != "McPlayer" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	want := []string{"exchange", "xbl", "xsts", "minecraft", "ownership", "profile"}
	if strings.Join(client.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected call order: %#v", client.calls)
	}
}

func TestMicrosoftAuthFlowRejectsMissingAccessToken(t *testing.T) {
	_, err := (microsoft.MicrosoftAuthFlow{Client: &missingAccessMicrosoftClient{}}).Complete(context.Background(), "auth_code")
	if err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing access_token error, got %v", err)
	}
}

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
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"ms_access"}`)),
				Header:     make(http.Header),
			}, nil
		})},
		ClientID:     "client_id",
		ClientSecret: "secret",
		RedirectURI:  "https://skin.example/microsoft/callback",
	}
	out, err := client.ExchangeCodeForToken(context.Background(), "auth_code")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "https://login.microsoftonline.com/consumers/oauth2/v2.0/token" || gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("unexpected token request shape: method=%s url=%s content-type=%s", gotMethod, gotURL, gotContentType)
	}
	if gotForm["client_id"] != "client_id" || gotForm["client_secret"] != "secret" || gotForm["code"] != "auth_code" ||
		gotForm["redirect_uri"] != "https://skin.example/microsoft/callback" || gotForm["grant_type"] != "authorization_code" {
		t.Fatalf("unexpected token request form: %#v", gotForm)
	}
	if out["access_token"] != "ms_access" {
		t.Fatalf("response should decode JSON exactly: %#v", out)
	}
}

func TestMicrosoftHTTPClientProfile404ReturnsNilProfile(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" || req.URL.String() != "https://api.minecraftservices.com/minecraft/profile" {
			t.Fatalf("unexpected profile request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer mc_access" {
			t.Fatalf("missing bearer auth: %q", req.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	out, err := client.GetMinecraftProfile(context.Background(), "mc_access")
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("profile 404 should decode as nil profile, got %#v", out)
	}
}

func TestMicrosoftHTTPClientRejectsNonSuccessWithStatusAndBody(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("upstream failed")), Header: make(http.Header)}, nil
	})}}
	_, err := client.CheckGameOwnership(context.Background(), "mc_access")
	if err == nil || !strings.Contains(err.Error(), "status=502") || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("expected status/body error, got %v", err)
	}
}

func TestMicrosoftHTTPClientXboxAndMinecraftRequestBodies(t *testing.T) {
	var seen []string
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body %s: %v", body, err)
		}
		seen = append(seen, req.URL.String())
		switch req.URL.String() {
		case "https://user.auth.xboxlive.com/user/authenticate":
			props := payload["Properties"].(map[string]any)
			if props["RpsTicket"] != "d=ms_access" || payload["RelyingParty"] != "http://auth.xboxlive.com" || payload["TokenType"] != "JWT" {
				t.Fatalf("unexpected XBL payload: %#v", payload)
			}
			return jsonResponse(`{"Token":"xbl_token","DisplayClaims":{"xui":[{"uhs":"user_hash"}]}}`), nil
		case "https://xsts.auth.xboxlive.com/xsts/authorize":
			props := payload["Properties"].(map[string]any)
			tokens := props["UserTokens"].([]any)
			if len(tokens) != 1 || tokens[0] != "xbl_token" || props["SandboxId"] != "RETAIL" || payload["RelyingParty"] != "rp://api.minecraftservices.com/" {
				t.Fatalf("unexpected XSTS payload: %#v", payload)
			}
			return jsonResponse(`{"Token":"xsts_token","DisplayClaims":{"xui":[{"uhs":"user_hash"}]}}`), nil
		case "https://api.minecraftservices.com/authentication/login_with_xbox":
			if payload["identityToken"] != "XBL3.0 x=user_hash;xsts_token" {
				t.Fatalf("unexpected Minecraft payload: %#v", payload)
			}
			return jsonResponse(`{"access_token":"mc_access"}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})}}

	xblToken, uhs, err := client.AuthenticateXBL(context.Background(), "ms_access")
	if err != nil || xblToken != "xbl_token" || uhs != "user_hash" {
		t.Fatalf("xbl got token=%q uhs=%q err=%v", xblToken, uhs, err)
	}
	xstsToken, uhs, err := client.AuthenticateXSTS(context.Background(), xblToken)
	if err != nil || xstsToken != "xsts_token" || uhs != "user_hash" {
		t.Fatalf("xsts got token=%q uhs=%q err=%v", xstsToken, uhs, err)
	}
	mcToken, err := client.AuthenticateMinecraft(context.Background(), uhs, xstsToken)
	if err != nil || mcToken != "mc_access" {
		t.Fatalf("minecraft got token=%q err=%v", mcToken, err)
	}
	want := strings.Join([]string{
		"https://user.auth.xboxlive.com/user/authenticate",
		"https://xsts.auth.xboxlive.com/xsts/authorize",
		"https://api.minecraftservices.com/authentication/login_with_xbox",
	}, ",")
	if strings.Join(seen, ",") != want {
		t.Fatalf("unexpected call sequence: %#v", seen)
	}
}

func TestMicrosoftHTTPClientRejectsMalformedXboxAndMinecraftResponses(t *testing.T) {
	xboxClient := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"DisplayClaims":{"xui":[]}}`), nil
	})}}
	if _, _, err := xboxClient.AuthenticateXBL(context.Background(), "ms_access"); err == nil || !strings.Contains(err.Error(), "missing token") {
		t.Fatalf("expected malformed xbox response error, got %v", err)
	}

	minecraftClient := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{}`), nil
	})}}
	if _, err := minecraftClient.AuthenticateMinecraft(context.Background(), "uhs", "xsts"); err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing minecraft access_token error, got %v", err)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
