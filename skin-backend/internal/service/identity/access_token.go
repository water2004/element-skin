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
	return s.accessTokenForOwnedIdentity(ctx, userID, identityID, false)
}

func (s Service) ForceRefreshAccessTokenForOwnedIdentity(ctx context.Context, userID, identityID string) (AuthorizedIdentity, error) {
	return s.accessTokenForOwnedIdentity(ctx, userID, identityID, true)
}

func (s Service) accessTokenForOwnedIdentity(ctx context.Context, userID, identityID string, forceRefresh bool) (AuthorizedIdentity, error) {
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
	credential, err := s.DB.Identities.GetCredential(ctx, identityID)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	if credential == nil {
		return AuthorizedIdentity{}, errors.New("external identity credential is missing")
	}
	if credential.AuthorizationStatus == model.ExternalIdentityAuthorizationReauthorizationRequired {
		return AuthorizedIdentity{}, conflict("external identity must be reauthorized")
	}
	if forceRefresh {
		if err := s.Redis.DeleteExternalAccessToken(ctx, identityID); err != nil {
			return AuthorizedIdentity{}, err
		}
	} else {
		cached, err := s.Redis.GetExternalAccessToken(ctx, identityID)
		if err == nil && cached.AccessToken != "" && cached.ExpiresAt > time.Now().Add(30*time.Second).UnixMilli() {
			return AuthorizedIdentity{Identity: *item, Provider: *provider, AccessToken: cached.AccessToken}, nil
		}
		if err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
			return AuthorizedIdentity{}, err
		}
	}

	value, err, _ := accessTokenRefreshes.Do(identityID, func() (any, error) {
		if !forceRefresh {
			cached, cacheErr := s.Redis.GetExternalAccessToken(ctx, identityID)
			if cacheErr == nil && cached.AccessToken != "" && cached.ExpiresAt > time.Now().Add(30*time.Second).UnixMilli() {
				return AuthorizedIdentity{Identity: *item, Provider: *provider, AccessToken: cached.AccessToken}, nil
			}
			if cacheErr != nil && !errors.Is(cacheErr, redisstore.ErrCacheMiss) {
				return AuthorizedIdentity{}, cacheErr
			}
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
		return AuthorizedIdentity{}, s.markReauthorizationRequired(ctx, item.ID, database.NowMS())
	}
	if credential.AuthorizationStatus == model.ExternalIdentityAuthorizationReauthorizationRequired {
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
		return AuthorizedIdentity{}, s.markReauthorizationRequired(ctx, item.ID, database.NowMS())
	}
	if err != nil {
		failedAt := database.NowMS()
		updated, updateErr := s.DB.Identities.MarkCredentialRefreshFailed(ctx, item.ID, failedAt)
		if updateErr != nil {
			return AuthorizedIdentity{}, updateErr
		}
		if !updated {
			return AuthorizedIdentity{}, errors.New("external identity credential is missing")
		}
		return AuthorizedIdentity{}, util.HTTPError{Status: 502, Detail: "external identity token refresh failed"}
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}
	if len(tokens.Scopes) == 0 {
		tokens.Scopes = append([]string(nil), credential.GrantedScopes...)
	}
	refreshedAt := database.NowMS()
	updatedCredential, err := s.credential(item.ID, tokens, refreshedAt)
	if err != nil {
		return AuthorizedIdentity{}, err
	}
	updatedCredential.LastRefreshAt = &refreshedAt
	if err := s.DB.Identities.UpdateCredential(ctx, updatedCredential); err != nil {
		return AuthorizedIdentity{}, err
	}
	if err := s.cacheAccessToken(ctx, item.ID, tokens); err != nil {
		return AuthorizedIdentity{}, err
	}
	return AuthorizedIdentity{Identity: item, Provider: provider, AccessToken: tokens.AccessToken}, nil
}

func (s Service) markReauthorizationRequired(ctx context.Context, identityID string, failedAt int64) error {
	updated, err := s.DB.Identities.MarkCredentialRefreshRejected(ctx, identityID, failedAt)
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("external identity credential is missing")
	}
	_ = s.Redis.DeleteExternalAccessToken(ctx, identityID)
	return conflict("external identity must be reauthorized")
}
