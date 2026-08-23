package oauth

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

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
	tokenActor, permissionActive, err := s.actorForAccessToken(ctx, access)
	if err != nil {
		return nil, err
	}
	oidcScopes := []string{}
	oidcActive := false
	if access.UserID != "" && hasOIDCScope(access.OIDCScopes, "openid") {
		grantScopes, active, err := s.DB.OAuth.ActiveGrantOIDCScopes(ctx, access.GrantID, access.UserID, access.ClientID)
		if err != nil {
			return nil, err
		}
		oidcScopes = intersectOIDCScopes(access.OIDCScopes, grantScopes)
		oidcActive = active && hasOIDCScope(oidcScopes, "openid")
	}
	if !permissionActive && !oidcActive {
		return map[string]any{"active": false}, nil
	}
	codes := []string{}
	if permissionActive {
		codes = permissionCodesFromBitSet(tokenActor.Permissions)
	}
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
		"scope":       strings.Join(combinedScopes(codes, oidcScopes), " "),
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
	return s.actorForAccessToken(ctx, token)
}

func (s Service) actorForAccessToken(ctx context.Context, token redisstore.OAuthAccessToken) (permission.Actor, bool, error) {
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
		if actor.Permissions.Empty() {
			return permission.Actor{}, false, nil
		}
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
	clientPermissionIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, token.ClientID)
	if err != nil {
		return permission.Actor{}, false, err
	}
	actor.SessionID = token.TokenHash
	actor.Permissions = actor.Permissions.
		And(bitSetFromPermissionIDs(token.PermissionIDs)).
		And(bitSetFromPermissionIDs(clientPermissionIDs))
	if actor.Permissions.Empty() {
		return permission.Actor{}, false, nil
	}
	return actor, true, nil
}
