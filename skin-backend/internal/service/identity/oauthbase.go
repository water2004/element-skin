package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"element-skin/backend/internal/util"
)

// Shared low-level OAuth2 primitives for providers that are not standard
// OpenID Connect relying parties (no discovery, no id_token, custom response
// formats). Vendor clients stay thin: they assemble parameters, call these
// helpers, and map results onto OIDCClaims/OIDCTokens.

const (
	oauthUserAgent    = "Element-Skin-OAuth2/1"
	oauthMaxBodyBytes = 1 << 20
	oauthHTTPTimeout  = 10 * time.Second
	oauthGrantCode    = "authorization_code"
)

// oauthTokenResult is the normalized outcome of an authorization-code exchange
// regardless of whether the upstream answered with JSON or the legacy
// application/x-www-form-urlencoded format. Fields keeps every scalar entry so
// vendor clients can read provider-specific extras such as QQ's openid.
type oauthTokenResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	Fields       map[string]string
}

// oauthUpstreamError marks a deterministic refusal returned by the upstream
// OAuth API (non-200 status). Transport faults and malformed payloads surface
// as plain errors instead.
type oauthUpstreamError struct {
	Status int
	Body   string
}

func (e oauthUpstreamError) Error() string {
	return fmt.Sprintf("upstream status %d body %.256q", e.Status, e.Body)
}

// classifyOAuthExchangeError maps primitive failures onto the stable
// identity_token.exchange triplet consumed by CompleteAuthorization.
func classifyOAuthExchangeError(err error) util.ClassifiedError {
	reason := "failed"
	var upstream oauthUpstreamError
	if errors.As(err, &upstream) {
		reason = "denied"
	}
	return util.ClassifiedError{Object: "identity_token", Operation: "exchange", Reason: reason, Cause: err}
}

// exchangeAuthorizationCode swaps an authorization code for a token set.
// method must be http.MethodGet or http.MethodPost: GET carries every
// parameter in the query string (QQ answers only this style), POST submits
// them as an application/x-www-form-urlencoded body.
func exchangeAuthorizationCode(ctx context.Context, client *http.Client, endpoint, method string, params url.Values) (oauthTokenResult, error) {
	req, err := newOAuthRequest(ctx, endpoint, method, params)
	if err != nil {
		return oauthTokenResult{}, err
	}
	resp, err := doOAuthRequest(ctx, client, req)
	if err != nil {
		return oauthTokenResult{}, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBodyBytes))
	if readErr != nil {
		return oauthTokenResult{}, fmt.Errorf("read token response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return oauthTokenResult{}, oauthUpstreamError{Status: resp.StatusCode, Body: string(body)}
	}
	result, parseErr := parseOAuthTokenResponse(body)
	if parseErr != nil {
		return oauthTokenResult{}, parseErr
	}
	return result, nil
}

// fetchOAuthProfileJSON retrieves a JSON profile document. When bearerToken is
// non-empty an Authorization header is attached; query parameters always ride
// along so query-style providers (QQ, WeChat, Gitee) work through one entry.
func fetchOAuthProfileJSON(ctx context.Context, client *http.Client, endpoint string, query url.Values, bearerToken string) (map[string]any, error) {
	req, err := newOAuthRequest(ctx, endpoint, http.MethodGet, query)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(bearerToken); trimmed != "" {
		req.Header.Set("Authorization", "Bearer "+trimmed)
	}
	resp, err := doOAuthRequest(ctx, client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, oauthMaxBodyBytes))
	if readErr != nil {
		return nil, fmt.Errorf("read profile response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, oauthUpstreamError{Status: resp.StatusCode, Body: string(body)}
	}
	var profile map[string]any
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}
	if profile == nil {
		return nil, errors.New("profile response is not a JSON object")
	}
	return profile, nil
}

func newOAuthRequest(ctx context.Context, endpoint, method string, params url.Values) (*http.Request, error) {
	var (
		req *http.Request
		err error
	)
	switch method {
	case http.MethodGet:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = params.Encode()
	case http.MethodPost:
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default:
		return nil, fmt.Errorf("unsupported OAuth request method %q", method)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", oauthUserAgent)
	return req, nil
}

func doOAuthRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = &http.Client{Timeout: oauthHTTPTimeout}
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("call %s %s: %w", req.Method, req.URL.Host, err)
	}
	return resp, nil
}

func parseOAuthTokenResponse(body []byte) (oauthTokenResult, error) {
	text := strings.TrimSpace(string(body))
	fields := map[string]string{}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		for key, value := range parsed {
			switch typed := value.(type) {
			case string:
				fields[key] = typed
			case float64:
				fields[key] = strconv.FormatInt(int64(typed), 10)
			case bool:
				fields[key] = strconv.FormatBool(typed)
			}
		}
	} else if values, formErr := url.ParseQuery(text); formErr == nil {
		for key, items := range values {
			if len(items) > 0 {
				fields[key] = items[0]
			}
		}
	} else {
		return oauthTokenResult{}, errors.New("token response is neither JSON nor form-encoded")
	}
	result := oauthTokenResult{Fields: fields}
	result.AccessToken = strings.TrimSpace(fields["access_token"])
	result.RefreshToken = fields["refresh_token"]
	result.TokenType = fields["token_type"]
	if rawExpires := strings.TrimSpace(fields["expires_in"]); rawExpires != "" {
		if seconds, convErr := strconv.ParseInt(rawExpires, 10, 64); convErr == nil && seconds > 0 {
			result.ExpiresIn = seconds
		}
	}
	return result, nil
}
