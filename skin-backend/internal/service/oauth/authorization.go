package oauth

import (
	"context"
	"net/url"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

const (
	GrantIssuanceGrace    = authorizationCodeTTL
	RevokedGrantRetention = 30 * 24 * time.Hour
)

type GrantCleanupResult struct {
	Revoked int64
	Deleted int64
}

var (
	oauthGrantRevokeSystemPermission = permission.MustDefinitionByCode("oauth_grant.revoke.system")
	oauthGrantDeleteSystemPermission = permission.MustDefinitionByCode("oauth_grant.delete.system")
)

func (s Service) ListGrants(ctx context.Context, actor permission.Actor, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_grant.read.owned")); err != nil {
		return nil, forbidden()
	}
	grants, err := s.DB.OAuth.ListGrantsByUser(ctx, actor.UserID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(grants))
	for _, grant := range grants {
		codes, err := s.grantPermissionCodes(ctx, grant.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, grantResponse(grant, codes))
	}
	return out, nil
}

func (s Service) RevokeGrant(ctx context.Context, actor permission.Actor, grantID string) error {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_grant.revoke.owned")); err != nil {
		return forbidden()
	}
	revokedAt := database.NowMS()
	ok, err := s.DB.OAuth.RevokeGrant(ctx, grantID, actor.UserID, revokedAt)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth grant not found")
	}
	return s.invalidateGrantCredentials(ctx, grantID, revokedAt)
}

func (s Service) CleanupGrants(ctx context.Context, actor permission.Actor, now int64) (GrantCleanupResult, error) {
	if err := actor.Require(oauthGrantRevokeSystemPermission); err != nil {
		return GrantCleanupResult{}, forbidden()
	}
	if err := actor.Require(oauthGrantDeleteSystemPermission); err != nil {
		return GrantCleanupResult{}, forbidden()
	}
	createdBefore := now - int64(GrantIssuanceGrace/time.Millisecond)
	revoked, err := s.DB.OAuth.RevokeInactiveGrants(ctx, now, createdBefore)
	if err != nil {
		return GrantCleanupResult{}, err
	}
	cutoff := now - int64(RevokedGrantRetention/time.Millisecond)
	deleted, err := s.DB.OAuth.DeleteRevokedGrants(ctx, cutoff)
	if err != nil {
		return GrantCleanupResult{Revoked: revoked}, err
	}
	return GrantCleanupResult{Revoked: revoked, Deleted: deleted}, nil
}

func (s Service) AuthorizationDetails(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (AuthorizationDetails, error) {
	client, scopes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return AuthorizationDetails{}, err
	}
	return AuthorizationDetails{
		Client:      publicClient(client),
		Scopes:      permissionDetails(scopes.Permissions),
		OIDCScopes:  scopes.OIDC,
		RedirectURI: req.RedirectURI,
		State:       req.State,
	}, nil
}

func (s Service) ApproveAuthorization(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (map[string]any, error) {
	client, scopes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	permissionIDs := permissionIDsFromCodes(scopes.Permissions)
	now := database.NowMS()
	rawCode, codeHash, err := generateToken()
	if err != nil {
		return nil, err
	}
	grantID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	grant := model.OAuthGrant{
		ID:         grantID,
		UserID:     actor.UserID,
		SubjectID:  permissiondb.SubjectIDForUser(actor.UserID),
		ClientID:   client.ID,
		OIDCScopes: scopes.OIDC,
		Status:     StatusActive,
		CreatedAt:  now,
	}
	code := model.OAuthAuthorizationCode{
		CodeHash:            codeHash,
		ClientID:            client.ID,
		UserID:              actor.UserID,
		GrantID:             grantID,
		RedirectURI:         req.RedirectURI,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: "S256",
		OIDCScopes:          scopes.OIDC,
		Nonce:               strings.TrimSpace(req.Nonce),
		ExpiresAt:           now + int64(authorizationCodeTTL/time.Millisecond),
		CreatedAt:           now,
	}
	if _, err := s.DB.OAuth.UpsertActiveGrantAndCreateAuthorizationCode(ctx, grant, permissionIDs, code); err != nil {
		return nil, err
	}
	redirectURL, err := authorizationRedirect(req.RedirectURI, rawCode, req.State)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"code":         rawCode,
		"redirect_url": redirectURL,
		"state":        req.State,
	}, nil
}

func (s Service) validAuthorizationRequest(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (model.OAuthClient, authorizationScopes, error) {
	if req.ResponseType != "code" {
		return model.OAuthClient{}, authorizationScopes{}, badRequest("response_type must be code")
	}
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(req.ClientID))
	if err != nil {
		return model.OAuthClient{}, authorizationScopes{}, err
	}
	if client == nil || client.Status != StatusActive {
		return model.OAuthClient{}, authorizationScopes{}, badRequest("invalid client_id")
	}
	if req.RedirectURI != client.RedirectURI {
		return model.OAuthClient{}, authorizationScopes{}, badRequest("invalid redirect_uri")
	}
	if req.CodeChallengeMethod != "S256" || strings.TrimSpace(req.CodeChallenge) == "" {
		return model.OAuthClient{}, authorizationScopes{}, badRequest("PKCE S256 is required")
	}
	if len(req.Nonce) > 512 {
		return model.OAuthClient{}, authorizationScopes{}, badRequest("nonce is too long")
	}
	scopes, err := parseAuthorizationScopes(req.Scope)
	if err != nil {
		return model.OAuthClient{}, authorizationScopes{}, err
	}
	clientIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		return model.OAuthClient{}, authorizationScopes{}, err
	}
	clientAllowed := idSet(clientIDs)
	for _, code := range scopes.Permissions {
		def := permission.MustDefinitionByCode(code)
		if def.Scope.ID == permission.ScopeServer {
			return model.OAuthClient{}, authorizationScopes{}, badRequest("invalid scope")
		}
		if !actor.Has(def) {
			return model.OAuthClient{}, authorizationScopes{}, forbidden()
		}
		if !clientAllowed[int64(def.ID)] {
			return model.OAuthClient{}, authorizationScopes{}, badRequest("scope exceeds client permission limit")
		}
	}
	return *client, scopes, nil
}

func (s Service) grantPermissionCodes(ctx context.Context, grantID string) ([]string, error) {
	ids, err := s.DB.OAuth.GrantPermissionIDs(ctx, grantID)
	if err != nil {
		return nil, err
	}
	return permissionCodesFromIDs(ids), nil
}

func (s Service) activeGrantPermissionCodes(ctx context.Context, grantID, userID, clientID string) ([]string, error) {
	ids, err := s.DB.OAuth.ActiveGrantPermissionIDs(ctx, grantID, userID, clientID)
	if err != nil {
		return nil, err
	}
	return permissionCodesFromIDs(ids), nil
}

func authorizationRedirect(rawURL, code, state string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", badRequest("invalid redirect_uri")
	}
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
