package oauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"sort"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

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
		return TokenResponse{}, badRequest("unsupported grant_type")
	}
}

func (s Service) RevokeToken(ctx context.Context, clientID, clientSecret, token string) error {
	client, err := s.authenticateClient(ctx, clientID, clientSecret)
	if err != nil {
		return err
	}
	tokenHash := util.HashRefreshToken(token)
	if access, err := s.Redis.GetOAuthAccessToken(ctx, tokenHash); err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		return err
	} else if err == nil {
		if access.ClientID != client.ID {
			return forbidden()
		}
		return s.Redis.DeleteOAuthAccessToken(ctx, tokenHash)
	}
	if refresh, err := s.DB.OAuth.GetRefreshToken(ctx, tokenHash); err != nil {
		return err
	} else if refresh != nil {
		if refresh.ClientID != client.ID {
			return forbidden()
		}
		_, err = s.DB.OAuth.RevokeRefreshToken(ctx, tokenHash, database.NowMS())
		return err
	}
	return nil
}

func (s Service) Introspect(ctx context.Context, actor permission.Actor, token string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_token.introspect.any")); err != nil {
		return nil, forbidden()
	}
	tokenHash := util.HashRefreshToken(token)
	access, err := s.Redis.GetOAuthAccessToken(ctx, tokenHash)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return map[string]any{"active": false}, nil
	}
	if err != nil {
		return nil, err
	}
	if access.ExpiresAt <= database.NowMS() {
		return map[string]any{"active": false}, nil
	}
	codes := permissionCodesFromIDs(access.PermissionIDs)
	if access.UserID == "" {
		return map[string]any{
			"active":      true,
			"client_id":   access.ClientID,
			"subject_id":  permissiondb.SubjectIDForClient(access.ClientID),
			"exp":         access.ExpiresAt / 1000,
			"scope":       strings.Join(codes, " "),
			"permissions": codes,
		}, nil
	}
	return map[string]any{
		"active":      true,
		"client_id":   access.ClientID,
		"user_id":     access.UserID,
		"grant_id":    access.GrantID,
		"exp":         access.ExpiresAt / 1000,
		"scope":       strings.Join(codes, " "),
		"permissions": codes,
	}, nil
}

func (s Service) ActorForBearer(ctx context.Context, bearer string) (permission.Actor, bool, error) {
	tokenHash := util.HashRefreshToken(bearer)
	token, err := s.Redis.GetOAuthAccessToken(ctx, tokenHash)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return permission.Actor{}, false, nil
	}
	if err != nil {
		return permission.Actor{}, false, err
	}
	if token.ExpiresAt <= database.NowMS() {
		return permission.Actor{}, false, nil
	}
	if token.UserID != "" {
		actor, err := s.DB.Permissions.ActorForUser(ctx, token.UserID, permissiondb.EffectiveOptions{
			SessionKind:       permission.SessionKindDelegated,
			Entrypoint:        permission.EntrypointDashboard,
			DelegatedClientID: token.ClientID,
			DelegatedGrantID:  token.GrantID,
		})
		if err != nil {
			return permission.Actor{}, false, err
		}
		actor.SessionID = token.TokenHash
		actor.Permissions = actor.Permissions.And(bitSetFromPermissionIDs(token.PermissionIDs))
		return actor, true, nil
	}

	client, err := s.DB.OAuth.GetClient(ctx, token.ClientID)
	if err != nil {
		return permission.Actor{}, false, err
	}
	if client == nil || client.Status != StatusActive {
		return permission.Actor{}, false, nil
	}
	actor, err := s.DB.Permissions.ActorForClient(ctx, token.ClientID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
	})
	if err != nil {
		return permission.Actor{}, false, err
	}
	actor.SessionID = token.TokenHash
	actor.Permissions = actor.Permissions.And(bitSetFromPermissionIDs(token.PermissionIDs))
	return actor, true, nil
}

func (s Service) exchangeAuthorizationCode(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	codeHash := util.HashRefreshToken(req.Code)
	code, permissionIDs, err := s.DB.OAuth.ConsumeAuthorizationCode(ctx, codeHash, database.NowMS())
	if err != nil {
		return TokenResponse{}, err
	}
	if code == nil || code.ClientID != client.ID || code.RedirectURI != req.RedirectURI {
		return TokenResponse{}, badRequest("invalid authorization code")
	}
	if !validPKCE(req.CodeVerifier, code.CodeChallenge) {
		return TokenResponse{}, badRequest("invalid code_verifier")
	}
	return s.issueTokens(ctx, client.ID, code.UserID, code.GrantID, permissionCodesFromIDs(permissionIDs))
}

func (s Service) refreshToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	oldHash := util.HashRefreshToken(req.RefreshToken)
	old, err := s.DB.OAuth.GetRefreshToken(ctx, oldHash)
	if err != nil {
		return TokenResponse{}, err
	}
	if old == nil || old.ClientID != client.ID || old.RevokedAt != nil || old.ExpiresAt <= database.NowMS() {
		return TokenResponse{}, badRequest("invalid refresh_token")
	}
	codes, err := s.grantPermissionCodes(ctx, old.GrantID)
	if err != nil {
		return TokenResponse{}, err
	}
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: client.ID, UserID: old.UserID, GrantID: old.GrantID, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
	ok, err := s.DB.OAuth.RotateRefreshToken(ctx, oldHash, refresh, now)
	if err != nil {
		return TokenResponse{}, err
	}
	if !ok {
		return TokenResponse{}, badRequest("invalid refresh_token")
	}
	if err := s.storeAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     accessHash,
		ClientID:      client.ID,
		UserID:        old.UserID,
		GrantID:       old.GrantID,
		PermissionIDs: permissionIDsFromCodes(codes),
		ExpiresAt:     now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt:     now,
	}); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(accessRaw, refreshRaw, codes), nil
}

func (s Service) clientCredentialsToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	if client.ClientType != ClientTypeConfidential {
		return TokenResponse{}, badRequest("client_credentials requires a confidential client")
	}
	actor, err := s.DB.Permissions.ActorForClient(ctx, client.ID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	codes, err := requestedOrDefaultClientScopes(req.Scope, actor)
	if err != nil {
		return TokenResponse{}, err
	}
	if len(codes) == 0 {
		return TokenResponse{}, forbidden()
	}
	raw, tokenHash, err := generateToken()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	if err := s.storeAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     tokenHash,
		ClientID:      client.ID,
		PermissionIDs: permissionIDsFromCodes(codes),
		ExpiresAt:     now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt:     now,
	}); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: raw,
		TokenType:   "Bearer",
		ExpiresIn:   int64(accessTokenTTL / time.Second),
		Scope:       strings.Join(codes, " "),
		Permissions: codes,
	}, nil
}

func (s Service) issueTokens(ctx context.Context, clientID, userID, grantID string, codes []string) (TokenResponse, error) {
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: clientID, UserID: userID, GrantID: grantID, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
	if err := s.DB.OAuth.CreateRefreshToken(ctx, refresh); err != nil {
		return TokenResponse{}, err
	}
	if err := s.storeAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     accessHash,
		ClientID:      clientID,
		UserID:        userID,
		GrantID:       grantID,
		PermissionIDs: permissionIDsFromCodes(codes),
		ExpiresAt:     now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt:     now,
	}); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(accessRaw, refreshRaw, codes), nil
}

func (s Service) storeAccessToken(ctx context.Context, token redisstore.OAuthAccessToken) error {
	return s.Redis.SetOAuthAccessToken(ctx, token, accessTokenTTL)
}

func (s Service) authenticateClient(ctx context.Context, clientID, secret string) (*model.OAuthClient, error) {
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(clientID))
	if err != nil {
		return nil, err
	}
	if client == nil || client.Status != StatusActive {
		return nil, badRequest("invalid client_id")
	}
	if client.ClientType == ClientTypeConfidential {
		if secret == "" || subtle.ConstantTimeCompare([]byte(client.SecretHash), []byte(util.HashRefreshToken(secret))) != 1 {
			return nil, badRequest("invalid client_secret")
		}
	}
	return client, nil
}

func requestedOrDefaultClientScopes(raw string, actor permission.Actor) ([]string, error) {
	if strings.TrimSpace(raw) != "" {
		codes, err := parseScope(raw)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			if !actor.Has(permission.MustDefinitionByCode(code)) {
				return nil, forbidden()
			}
		}
		return codes, nil
	}
	codes := make([]string, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if actor.Has(def) {
			codes = append(codes, def.Code)
		}
	}
	sort.Strings(codes)
	return codes, nil
}
