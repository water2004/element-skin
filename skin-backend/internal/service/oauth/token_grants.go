package oauth

import (
	"context"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (s Service) exchangeAuthorizationCode(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	codeHash := util.HashRefreshToken(req.Code)
	code, codePermissionIDs, err := s.DB.OAuth.ConsumeAuthorizationCode(ctx, codeHash, client.ID, req.RedirectURI, database.NowMS())
	if err != nil {
		return TokenResponse{}, err
	}
	if code == nil {
		return TokenResponse{}, badRequest("invalid authorization code")
	}
	if !validPKCE(req.CodeVerifier, code.CodeChallenge) {
		return TokenResponse{}, badRequest("invalid code_verifier")
	}
	activeCodes, err := s.activeGrantPermissionCodes(ctx, code.GrantID, code.UserID, client.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	grantOIDCScopes, active, err := s.DB.OAuth.ActiveGrantOIDCScopes(ctx, code.GrantID, code.UserID, client.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	codes := intersectPermissionCodes(permissionCodesFromIDs(codePermissionIDs), activeCodes)
	oidcScopes := intersectOIDCScopes(code.OIDCScopes, grantOIDCScopes)
	if !active || (len(codes) == 0 && len(oidcScopes) == 0) {
		return TokenResponse{}, badRequest("invalid authorization code")
	}
	return s.issueTokens(ctx, client.ID, code.UserID, code.GrantID, codes, oidcScopes, code.Nonce)
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
	codes, err := s.activeGrantPermissionCodes(ctx, old.GrantID, old.UserID, client.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	grantOIDCScopes, active, err := s.DB.OAuth.ActiveGrantOIDCScopes(ctx, old.GrantID, old.UserID, client.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	oidcScopes := intersectOIDCScopes(old.OIDCScopes, grantOIDCScopes)
	if !active || (len(codes) == 0 && len(oidcScopes) == 0) {
		return TokenResponse{}, badRequest("invalid refresh_token")
	}
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	idToken, err := s.issueIDToken(ctx, client.ID, old.UserID, "", oidcScopes, now)
	if err != nil {
		return TokenResponse{}, err
	}
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: client.ID, UserID: old.UserID, GrantID: old.GrantID, OIDCScopes: oidcScopes, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
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
		OIDCScopes:    oidcScopes,
		ExpiresAt:     now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt:     now,
	}); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(accessRaw, refreshRaw, codes, oidcScopes, idToken), nil
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
	clientPermissionIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	actor.Permissions = actor.Permissions.And(bitSetFromPermissionIDs(clientPermissionIDs))
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

func (s Service) issueTokens(ctx context.Context, clientID, userID, grantID string, codes, oidcScopes []string, nonce string) (TokenResponse, error) {
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	idToken, err := s.issueIDToken(ctx, clientID, userID, nonce, oidcScopes, now)
	if err != nil {
		return TokenResponse{}, err
	}
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: clientID, UserID: userID, GrantID: grantID, OIDCScopes: oidcScopes, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
	if err := s.DB.OAuth.CreateRefreshToken(ctx, refresh); err != nil {
		return TokenResponse{}, err
	}
	if err := s.storeAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     accessHash,
		ClientID:      clientID,
		UserID:        userID,
		GrantID:       grantID,
		PermissionIDs: permissionIDsFromCodes(codes),
		OIDCScopes:    oidcScopes,
		ExpiresAt:     now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt:     now,
	}); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(accessRaw, refreshRaw, codes, oidcScopes, idToken), nil
}

func (s Service) storeAccessToken(ctx context.Context, token redisstore.OAuthAccessToken) error {
	return s.Redis.SetOAuthAccessToken(ctx, token, accessTokenTTL)
}
