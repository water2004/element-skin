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
	ok, err := s.DB.OAuth.RevokeGrant(ctx, grantID, actor.UserID, database.NowMS())
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth grant not found")
	}
	return nil
}

func (s Service) AuthorizationDetails(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (AuthorizationDetails, error) {
	client, codes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return AuthorizationDetails{}, err
	}
	return AuthorizationDetails{
		Client:      publicClient(client),
		Scopes:      permissionDetails(codes),
		RedirectURI: req.RedirectURI,
		State:       req.State,
	}, nil
}

func (s Service) ApproveAuthorization(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (map[string]any, error) {
	client, codes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	permissionIDs := permissionIDsFromCodes(codes)
	now := database.NowMS()
	grantID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	grant := model.OAuthGrant{
		ID:        grantID,
		UserID:    actor.UserID,
		SubjectID: permissiondb.SubjectIDForUser(actor.UserID),
		ClientID:  client.ID,
		Status:    StatusActive,
		CreatedAt: now,
	}
	if err := s.DB.OAuth.CreateGrant(ctx, grant, permissionIDs); err != nil {
		return nil, err
	}
	rawCode, codeHash, err := generateToken()
	if err != nil {
		return nil, err
	}
	code := model.OAuthAuthorizationCode{
		CodeHash:            codeHash,
		ClientID:            client.ID,
		UserID:              actor.UserID,
		GrantID:             grantID,
		RedirectURI:         req.RedirectURI,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           now + int64(authorizationCodeTTL/time.Millisecond),
		CreatedAt:           now,
	}
	if err := s.DB.OAuth.CreateAuthorizationCode(ctx, code, permissionIDs); err != nil {
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

func (s Service) validAuthorizationRequest(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (model.OAuthClient, []string, error) {
	if req.ResponseType != "code" {
		return model.OAuthClient{}, nil, badRequest("response_type must be code")
	}
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(req.ClientID))
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	if client == nil || client.Status != StatusActive {
		return model.OAuthClient{}, nil, badRequest("invalid client_id")
	}
	if req.RedirectURI != client.RedirectURI {
		return model.OAuthClient{}, nil, badRequest("invalid redirect_uri")
	}
	if req.CodeChallengeMethod != "S256" || strings.TrimSpace(req.CodeChallenge) == "" {
		return model.OAuthClient{}, nil, badRequest("PKCE S256 is required")
	}
	codes, err := parseScope(req.Scope)
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	clientIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	clientAllowed := idSet(clientIDs)
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		if !actor.Has(def) {
			return model.OAuthClient{}, nil, forbidden()
		}
		if !clientAllowed[int64(def.ID)] {
			return model.OAuthClient{}, nil, badRequest("scope exceeds client permission limit")
		}
	}
	return *client, codes, nil
}

func (s Service) grantPermissionCodes(ctx context.Context, grantID string) ([]string, error) {
	ids, err := s.DB.OAuth.GrantPermissionIDs(ctx, grantID)
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
