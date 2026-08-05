package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/model"

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
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC token exchange failed")
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC token response did not include an ID token")
	}
	verifier := oidc.NewVerifier(provider.IssuerURL, oidc.NewRemoteKeySet(ctx, provider.JWKSURI), &oidc.Config{ClientID: provider.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC ID token verification failed")
	}
	if idToken.Nonce != expectedNonce {
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC ID token nonce mismatch")
	}
	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC ID token claims are invalid")
	}
	claims := OIDCClaims{
		Subject:       stringClaim(rawClaims, "sub"),
		Email:         stringClaim(rawClaims, "email"),
		EmailVerified: boolClaim(rawClaims, "email_verified"),
		DisplayName:   firstStringClaim(rawClaims, "name", "preferred_username"),
		AvatarURL:     stringClaim(rawClaims, "picture"),
	}
	if claims.Subject == "" {
		return OIDCClaims{}, OIDCTokens{}, errors.New("OIDC ID token subject is missing")
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
