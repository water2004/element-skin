package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"

	"golang.org/x/sync/singleflight"
)

var accessTokenRefreshes singleflight.Group

type AuthorizedIdentity struct {
	Identity    model.ExternalIdentity
	Provider    model.IdentityProvider
	AccessToken string
}

func (s Service) AccessTokenForOwnedIdentity(ctx context.Context, userID, identityID string) (AuthorizedIdentity, error) {
	userID = strings.TrimSpace(userID)
	identityID = strings.TrimSpace(identityID)
	if userID == "" || identityID == "" {
		return AuthorizedIdentity{}, badRequest("identity_id is required")
	}
	item, err := s.DB.Identities.GetIdentity(ctx, identityID)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	if item == nil || item.UserID != userID {
		return AuthorizedIdentity{}, notFound("external identity not found")
	}
	provider, err := s.DB.Identities.GetProvider(ctx, item.ProviderID)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	if provider == nil || !provider.Enabled {
		return AuthorizedIdentity{}, conflict("external identity provider is unavailable")
	}

	cached, err := s.Redis.GetExternalAccessToken(ctx, identityID)
	if err == nil && cached.AccessToken != "" && cached.ExpiresAt > time.Now().Add(30*time.Second).UnixMilli() {
		return AuthorizedIdentity{Identity: *item, Provider: *provider, AccessToken: cached.AccessToken}, nil
	}
	if err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		return AuthorizedIdentity{}, err
	}
	value, err, _ := accessTokenRefreshes.Do(identityID, func() (any, error) {
		cached, cacheErr := s.Redis.GetExternalAccessToken(ctx, identityID)
		if cacheErr == nil && cached.AccessToken != "" && cached.ExpiresAt > time.Now().Add(30*time.Second).UnixMilli() {
			return AuthorizedIdentity{Identity: *item, Provider: *provider, AccessToken: cached.AccessToken}, nil
		}
		if cacheErr != nil && !errors.Is(cacheErr, redisstore.ErrCacheMiss) {
			return AuthorizedIdentity{}, cacheErr
		}
		return s.refreshAccessToken(ctx, *item, *provider)
	})
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	return value.(AuthorizedIdentity), nil
}

func (s Service) refreshAccessToken(ctx context.Context, item model.ExternalIdentity, provider model.IdentityProvider) (AuthorizedIdentity, error) {
	credential, err := s.DB.Identities.GetCredential(ctx, item.ID)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	if credential == nil || credential.RefreshTokenCiphertext == "" {
		return AuthorizedIdentity{}, conflict("external identity must be reauthorized")
	}
	box, err := util.NewSecretBox(s.Config.IdentityEncryptionKey)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	refreshToken, err := box.Decrypt(credential.RefreshTokenCiphertext)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	clientSecret, err := box.Decrypt(provider.ClientSecretCiphertext)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	refresher := s.TokenRefresher
	if refresher == nil {
		refresher = StandardOIDCClient{}
	}
	tokens, err := refresher.Refresh(ctx, provider, clientSecret, refreshToken, credential.GrantedScopes)
	if errors.Is(err, ErrRefreshRejected) {
		return AuthorizedIdentity{}, conflict("external identity must be reauthorized")
	}
	if err != nil {
		return AuthorizedIdentity{}, util.HTTPError{Status: 502, Detail: "external identity token refresh failed"}
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}
	if len(tokens.Scopes) == 0 {
		tokens.Scopes = append([]string(nil), credential.GrantedScopes...)
	}
	updatedCredential, err := s.credential(item.ID, tokens, database.NowMS())
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	if err := s.DB.Identities.UpdateCredential(ctx, updatedCredential); err != nil {
		return AuthorizedIdentity{}, err
	}
	if err := s.cacheAccessToken(ctx, item.ID, tokens); err != nil {
		return AuthorizedIdentity{}, err
	}
	return AuthorizedIdentity{Identity: item, Provider: provider, AccessToken: tokens.AccessToken}, nil
}
