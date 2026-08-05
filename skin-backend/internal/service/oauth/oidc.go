package oauth

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (s Service) issueIDToken(ctx context.Context, clientID, userID, nonce string, scopes []string, now int64) (string, error) {
	if !hasOIDCScope(scopes, "openid") {
		return "", nil
	}
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("OIDC token references a missing user")
	}
	subject, err := s.pairwiseSubject(ctx, clientID, userID)
	if err != nil {
		return "", err
	}
	claims := s.userClaims(*user, subject, scopes)
	claims["iss"] = s.issuer()
	claims["aud"] = clientID
	claims["iat"] = now / 1000
	claims["exp"] = now/1000 + int64(accessTokenTTL.Seconds())
	if nonce != "" {
		claims["nonce"] = nonce
	}
	return s.OIDCSigner.Sign(claims)
}

func (s Service) UserInfo(ctx context.Context, bearer string) (map[string]any, error) {
	if s.Redis == nil {
		return nil, errors.New("OAuth token store is not configured")
	}
	token, err := s.Redis.GetOAuthAccessToken(ctx, util.HashRefreshToken(strings.TrimSpace(bearer)))
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, unauthorized("invalid access token")
	}
	if err != nil {
		return nil, err
	}
	if token.UserID == "" || token.GrantID == "" || token.ExpiresAt <= database.NowMS() || !hasOIDCScope(token.OIDCScopes, "openid") {
		return nil, unauthorized("invalid access token")
	}
	grantScopes, active, err := s.DB.OAuth.ActiveGrantOIDCScopes(ctx, token.GrantID, token.UserID, token.ClientID)
	if err != nil {
		return nil, err
	}
	if !active || !hasOIDCScope(grantScopes, "openid") {
		return nil, unauthorized("invalid access token")
	}
	user, err := s.DB.Users.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, unauthorized("invalid access token")
	}
	subject, err := s.DB.OAuth.GetPairwiseSubject(ctx, token.ClientID, token.UserID)
	if err != nil {
		return nil, err
	}
	if subject == "" {
		return nil, unauthorized("invalid access token")
	}
	return s.userClaims(*user, subject, intersectOIDCScopes(token.OIDCScopes, grantScopes)), nil
}

func (s Service) pairwiseSubject(ctx context.Context, clientID, userID string) (string, error) {
	if existing, err := s.DB.OAuth.GetPairwiseSubject(ctx, clientID, userID); err != nil || existing != "" {
		return existing, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		subject, _, err := util.GenerateRefreshToken()
		if err != nil {
			return "", err
		}
		stored, err := s.DB.OAuth.CreatePairwiseSubject(ctx, clientID, userID, subject, database.NowMS())
		if err == nil {
			return stored, nil
		}
		if attempt == 2 {
			return "", err
		}
	}
	return "", errors.New("failed to create OIDC pairwise subject")
}

func (s Service) userClaims(user model.User, subject string, scopes []string) map[string]any {
	claims := map[string]any{"sub": subject}
	if hasOIDCScope(scopes, "profile") {
		claims["name"] = user.DisplayName
		claims["preferred_username"] = user.DisplayName
		if user.PreferredLanguage != "" {
			claims["locale"] = user.PreferredLanguage
		}
	}
	if hasOIDCScope(scopes, "email") {
		claims["email"] = user.Email
		claims["email_verified"] = false
	}
	return claims
}

func (s Service) issuer() string {
	if base := strings.TrimRight(strings.TrimSpace(s.Config.APIURL), "/"); base != "" {
		return base
	}
	return strings.TrimRight(strings.TrimSpace(s.Config.SiteURL), "/")
}

func intersectOIDCScopes(left, right []string) []string {
	allowed := make(map[string]bool, len(right))
	for _, scope := range right {
		allowed[scope] = true
	}
	out := make([]string, 0, len(left))
	for _, scope := range left {
		if allowed[scope] {
			out = append(out, scope)
		}
	}
	return out
}
