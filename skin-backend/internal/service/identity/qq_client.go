package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

// Dedicated adapter for QQ 互联 (graph.qq.com). The protocol is OAuth2-shaped
// but not OpenID Connect: no discovery, no id_token, no signature to verify.
// Endpoints and scopes are platform constants — administrators only provide
// the app credentials. Tokens are deliberately not persisted: QQ identities
// exist for login authentication only, so ExchangeAndVerify returns empty
// tokens and the service layer stores an empty credential row.
const (
	QQUIssuerURL            = "https://graph.qq.com"
	QQAuthorizationEndpoint = QQUIssuerURL + "/oauth2.0/authorize"
	QQTokenEndpoint         = QQUIssuerURL + "/oauth2.0/token"
	QQUserInfoEndpoint      = QQUIssuerURL + "/user/get_user_info"
)

var QQScopes = []string{"get_user_info"}

type QQClient struct {
	HTTPClient *http.Client
}

func (c QQClient) ExchangeAndVerify(
	ctx context.Context,
	provider model.IdentityProvider,
	clientSecret string,
	code string,
	redirectURI string,
	_ string,
	_ string,
) (OIDCClaims, OIDCTokens, error) {
	token, err := exchangeAuthorizationCode(ctx, c.HTTPClient, provider.TokenEndpoint, http.MethodGet, url.Values{
		"grant_type":    {oauthGrantCode},
		"client_id":     {provider.ClientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"fmt":           {"json"},
		"need_openid":   {"1"},
	})
	if err != nil {
		return OIDCClaims{}, OIDCTokens{}, classifyOAuthExchangeError(err)
	}
	// The official Connect2.1 PHP SDK shows this endpoint can answer an error
	// payload wrapped in callback(...) with HTTP 200; detect it explicitly.
	if errorCode := strings.TrimSpace(token.Fields["error"]); errorCode != "" {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{
			Object: "identity_token", Operation: "exchange", Reason: "denied",
			Cause: fmt.Errorf("token endpoint rejected: error=%s description=%q",
				errorCode, token.Fields["error_description"]),
		}
	}
	subject := strings.TrimSpace(token.Fields["openid"])
	if subject == "" {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{
			Object: "identity_token", Operation: "exchange", Reason: "incomplete",
			Cause: errors.New("qq token response carries no openid"),
		}
	}
	profile, err := fetchOAuthProfileJSON(ctx, c.HTTPClient, provider.UserInfoEndpoint, url.Values{
		"access_token":       {token.AccessToken},
		"oauth_consumer_key": {provider.ClientID},
		"openid":             {subject},
		"format":             {"json"},
	}, "")
	if err != nil {
		return OIDCClaims{}, OIDCTokens{}, classifyOAuthExchangeError(err)
	}
	if ret, ok := profile["ret"].(float64); ok && ret != 0 {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{
			Object: "identity_token", Operation: "exchange", Reason: "denied",
			Cause: fmt.Errorf("get_user_info failed ret=%v msg=%q", int64(ret), profile["msg"]),
		}
	}
	claims := OIDCClaims{
		Subject:     subject,
		DisplayName: stringClaim(profile, "nickname"),
		AvatarURL:   firstStringClaim(profile, "figureurl_qq_2", "figureurl_qq_1"),
	}
	return claims, OIDCTokens{Scopes: append([]string(nil), provider.Scopes...)}, nil
}
