package microsoft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"element-skin/backend/internal/util"
)

func MicrosoftAuthorizationURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "XboxLive.signin offline_access")
	q.Set("state", state)
	return "https://login.live.com/oauth20_authorize.srf?" + q.Encode()
}

type MicrosoftAuthClient interface {
	ExchangeCodeForToken(ctx context.Context, code string) (map[string]any, error)
	AuthenticateXBL(ctx context.Context, msAccessToken string) (token string, userHash string, err error)
	AuthenticateXSTS(ctx context.Context, xblToken string) (token string, userHash string, err error)
	AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error)
	CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error)
	GetMinecraftProfile(ctx context.Context, mcAccessToken string) (map[string]any, error)
}

type MicrosoftAuthFlow struct {
	Client MicrosoftAuthClient
}

func (f MicrosoftAuthFlow) Complete(ctx context.Context, code string) (map[string]any, error) {
	tokenData, err := f.Client.ExchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, err
	}
	msAccess, _ := tokenData["access_token"].(string)
	if msAccess == "" {
		return nil, util.HTTPError{Status: 400, Detail: "Microsoft token response missing access_token"}
	}
	xblToken, _, err := f.Client.AuthenticateXBL(ctx, msAccess)
	if err != nil {
		return nil, err
	}
	xstsToken, userHash, err := f.Client.AuthenticateXSTS(ctx, xblToken)
	if err != nil {
		return nil, err
	}
	mcAccess, err := f.Client.AuthenticateMinecraft(ctx, userHash, xstsToken)
	if err != nil {
		return nil, err
	}
	hasGame, err := f.Client.CheckGameOwnership(ctx, mcAccess)
	if err != nil {
		return nil, err
	}
	profile, err := f.Client.GetMinecraftProfile(ctx, mcAccess)
	if err != nil {
		return nil, err
	}
	return map[string]any{"mc_access_token": mcAccess, "has_game": hasGame, "profile": profile}, nil
}

type MicrosoftHTTPClient struct {
	Client       *http.Client
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func (c MicrosoftHTTPClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c MicrosoftHTTPClient) ExchangeCodeForToken(ctx context.Context, code string) (map[string]any, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.RedirectURI)
	form.Set("grant_type", "authorization_code")
	var out map[string]any
	err := c.do(ctx, "POST", "https://login.microsoftonline.com/consumers/oauth2/v2.0/token", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", "", &out)
	return out, err
}

func (c MicrosoftHTTPClient) AuthenticateXBL(ctx context.Context, msAccessToken string) (string, string, error) {
	var out map[string]any
	err := c.postJSON(ctx, "https://user.auth.xboxlive.com/user/authenticate", map[string]any{
		"Properties":   map[string]any{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": "d=" + msAccessToken},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}, "", &out)
	if err != nil {
		return "", "", err
	}
	return tokenAndUHS(out)
}

func (c MicrosoftHTTPClient) AuthenticateXSTS(ctx context.Context, xblToken string) (string, string, error) {
	var out map[string]any
	err := c.postJSON(ctx, "https://xsts.auth.xboxlive.com/xsts/authorize", map[string]any{
		"Properties":   map[string]any{"SandboxId": "RETAIL", "UserTokens": []string{xblToken}},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}, "", &out)
	if err != nil {
		return "", "", err
	}
	return tokenAndUHS(out)
}

func (c MicrosoftHTTPClient) AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error) {
	var out map[string]any
	if err := c.postJSON(ctx, "https://api.minecraftservices.com/authentication/login_with_xbox", map[string]any{
		"identityToken": "XBL3.0 x=" + userHash + ";" + xstsToken,
	}, "", &out); err != nil {
		return "", err
	}
	token, _ := out["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("minecraft login response missing access_token")
	}
	return token, nil
}

func (c MicrosoftHTTPClient) CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error) {
	var out map[string]any
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/entitlements/mcstore", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return false, err
	}
	items, _ := out["items"].([]any)
	return len(items) > 0, nil
}

func (c MicrosoftHTTPClient) GetMinecraftProfile(ctx context.Context, mcAccessToken string) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/minecraft/profile", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c MicrosoftHTTPClient) postJSON(ctx context.Context, endpoint string, body any, bearer string, out any) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return err
	}
	return c.do(ctx, "POST", endpoint, &b, "application/json", bearer, out)
}

func (c MicrosoftHTTPClient) do(ctx context.Context, method, endpoint string, body io.Reader, contentType, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 && method == "GET" && strings.Contains(endpoint, "/minecraft/profile") {
		if ptr, ok := out.(*map[string]any); ok {
			*ptr = nil
		}
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("microsoft request failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func tokenAndUHS(data map[string]any) (string, string, error) {
	token, _ := data["Token"].(string)
	claims, _ := data["DisplayClaims"].(map[string]any)
	xui, _ := claims["xui"].([]any)
	if token == "" || len(xui) == 0 {
		return "", "", fmt.Errorf("xbox response missing token or user hash")
	}
	first, _ := xui[0].(map[string]any)
	uhs, _ := first["uhs"].(string)
	if uhs == "" {
		return "", "", fmt.Errorf("xbox response missing user hash")
	}
	return token, uhs, nil
}
