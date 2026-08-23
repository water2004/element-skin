package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

type OIDCTokens struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	Scopes       []string
}

type OIDCClient interface {
	ExchangeAndVerify(context.Context, model.IdentityProvider, string, string, string, string, string) (OIDCClaims, OIDCTokens, error)
}

type TokenRefresher interface {
	Refresh(context.Context, model.IdentityProvider, string, string, []string) (OIDCTokens, error)
}

var ErrRefreshRejected = errors.New("external identity refresh token was rejected")

type StandardOIDCClient struct {
	HTTPClient *http.Client
}

func (c StandardOIDCClient) ExchangeAndVerify(
	ctx context.Context,
	provider model.IdentityProvider,
	clientSecret string,
	code string,
	redirectURI string,
	pkceVerifier string,
	expectedNonce string,
) (OIDCClaims, OIDCTokens, error) {
	if c.HTTPClient != nil {
		ctx = oidc.ClientContext(ctx, c.HTTPClient)
	}
	config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL:  provider.AuthorizationEndpoint,
			TokenURL: provider.TokenEndpoint,
		},
		Scopes: provider.Scopes,
	}
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "identity_token", Operation: "exchange", Reason: "failed", Cause: err}
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "id_token", Operation: "exchange", Reason: "required"}
	}
	verifier := oidc.NewVerifier(provider.IssuerURL, oidc.NewRemoteKeySet(ctx, provider.JWKSURI), &oidc.Config{ClientID: provider.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "id_token", Operation: "verify", Reason: "invalid", Cause: err}
	}
	if idToken.Nonce != expectedNonce {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "oidc_nonce", Operation: "verify", Reason: "mismatch"}
	}
	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "id_token", Operation: "verify", Reason: "invalid", Cause: err}
	}
	claims := OIDCClaims{
		Subject:       stringClaim(rawClaims, "sub"),
		Email:         stringClaim(rawClaims, "email"),
		EmailVerified: boolClaim(rawClaims, "email_verified"),
		DisplayName:   firstStringClaim(rawClaims, "name", "preferred_username"),
		AvatarURL:     stringClaim(rawClaims, "picture"),
	}
	if claims.Subject == "" {
		return OIDCClaims{}, OIDCTokens{}, util.ClassifiedError{Object: "id_token", Operation: "verify", Reason: "incomplete"}
	}
	scopes := append([]string(nil), provider.Scopes...)
	if granted, ok := token.Extra("scope").(string); ok && strings.TrimSpace(granted) != "" {
		scopes = strings.Fields(granted)
	}
	return claims, OIDCTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
		Scopes:       scopes,
	}, nil
}

func (c StandardOIDCClient) Refresh(
	ctx context.Context,
	provider model.IdentityProvider,
	clientSecret string,
	refreshToken string,
	scopes []string,
) (OIDCTokens, error) {
	if c.HTTPClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.HTTPClient)
	}
	config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: provider.TokenEndpoint,
		},
		Scopes: scopes,
	}
	token, err := config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(-time.Minute),
	}).Token()
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && (retrieveErr.ErrorCode == "invalid_grant" || retrieveErr.ErrorCode == "invalid_request") {
			return OIDCTokens{}, ErrRefreshRejected
		}
		return OIDCTokens{}, util.ClassifiedError{Object: "identity_token", Operation: "refresh", Reason: "failed", Cause: err}
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return OIDCTokens{}, util.ClassifiedError{Object: "access_token", Operation: "exchange", Reason: "required"}
	}
	grantedScopes := append([]string(nil), scopes...)
	if granted, ok := token.Extra("scope").(string); ok && strings.TrimSpace(granted) != "" {
		grantedScopes = strings.Fields(granted)
	}
	return OIDCTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
		Scopes:       grantedScopes,
	}, nil
}

func stringClaim(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}

func firstStringClaim(claims map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringClaim(claims, key); value != "" {
			return value
		}
	}
	return ""
}

func boolClaim(claims map[string]any, key string) bool {
	value, _ := claims[key].(bool)
	return value
}
