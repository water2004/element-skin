package oauth

import "context"

func (s Service) IssueToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	switch req.GrantType {
	case "authorization_code":
		return s.exchangeAuthorizationCode(ctx, req)
	case "refresh_token":
		return s.refreshToken(ctx, req)
	case "client_credentials":
		return s.clientCredentialsToken(ctx, req)
	case "urn:ietf:params:oauth:grant-type:device_code":
		return s.deviceCodeToken(ctx, req)
	default:
		return TokenResponse{}, badRequest("grant_type", "validate", "unsupported")
	}
}
