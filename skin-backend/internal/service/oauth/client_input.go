package oauth

import (
	"context"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
)

func (s Service) clientFromInput(actor permission.Actor, input ClientInput) (model.OAuthClient, []int64, []string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return model.OAuthClient{}, nil, nil, badRequest("invalid name")
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if redirectURI != "" && !validHTTPURL(redirectURI) {
		return model.OAuthClient{}, nil, nil, badRequest("invalid redirect_uri")
	}
	websiteURL := strings.TrimSpace(input.WebsiteURL)
	if websiteURL != "" && !validHTTPURL(websiteURL) {
		return model.OAuthClient{}, nil, nil, badRequest("invalid website_url")
	}
	clientType := strings.TrimSpace(input.ClientType)
	if clientType == "" {
		clientType = ClientTypeConfidential
	}
	if clientType != ClientTypeConfidential && clientType != ClientTypePublic {
		return model.OAuthClient{}, nil, nil, badRequest("invalid client_type")
	}
	codes, err := validateCodes(input.PermissionCodes)
	if err != nil {
		return model.OAuthClient{}, nil, nil, err
	}
	ids := permissionIDsFromCodes(codes)
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		if def.Scope.ID == permission.ScopeServer {
			if clientType != ClientTypeConfidential {
				return model.OAuthClient{}, nil, nil, badRequest("server scope requires confidential client")
			}
			continue
		}
		if !actor.Has(def) {
			return model.OAuthClient{}, nil, nil, forbidden()
		}
	}
	client := model.OAuthClient{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		RedirectURI: redirectURI,
		WebsiteURL:  websiteURL,
		ClientType:  clientType,
	}
	return client, ids, codes, nil
}

func (s Service) clientForActor(ctx context.Context, actor permission.Actor, clientID, ownedCode, anyCode string) (*model.OAuthClient, error) {
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	if actor.Has(permission.MustDefinitionByCode(anyCode)) {
		return client, nil
	}
	if client.OwnerUserID == actor.UserID && actor.Has(permission.MustDefinitionByCode(ownedCode)) {
		return client, nil
	}
	return nil, forbidden()
}

func (s Service) clientPermissionCodes(ctx context.Context, clientID string) ([]string, error) {
	ids, err := s.DB.OAuth.ClientPermissionIDs(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return permissionCodesFromIDs(ids), nil
}
