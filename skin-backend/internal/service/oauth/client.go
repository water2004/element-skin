package oauth

import (
	"context"
	"strings"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) CreateClient(ctx context.Context, actor permission.Actor, input ClientInput) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.create.owned")); err != nil {
		return nil, forbidden()
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	client.ID, err = util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	client.OwnerUserID = actor.UserID
	client.Status = StatusPending
	client.CreatedAt = database.NowMS()
	client.UpdatedAt = client.CreatedAt
	secret := ""
	if client.ClientType == ClientTypeConfidential {
		secret, client.SecretHash, err = generateSecret()
		if err != nil {
			return nil, err
		}
	}
	if err := s.DB.OAuth.CreateClient(ctx, client, permissionIDs); err != nil {
		return nil, err
	}
	return clientResponse(client, permissionCodes, secret), nil
}

func (s Service) ListClients(ctx context.Context, actor permission.Actor, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.read.owned")); err != nil {
		return nil, forbidden()
	}
	clients, err := s.DB.OAuth.ListClientsByOwner(ctx, actor.UserID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		codes, err := s.clientPermissionCodes(ctx, client.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, clientResponse(client, codes, ""))
	}
	return out, nil
}

func (s Service) ListClientsForAdmin(ctx context.Context, actor permission.Actor, status string, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.read.any")); err != nil {
		return nil, forbidden()
	}
	status = strings.TrimSpace(status)
	if status != "" && status != "all" && !validClientStatus(status) {
		return nil, badRequest("invalid status")
	}
	clients, err := s.DB.OAuth.ListClientsByStatus(ctx, status, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		codes, err := s.clientPermissionCodes(ctx, client.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, clientResponse(client, codes, ""))
	}
	return out, nil
}

func (s Service) GetClient(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.read.owned", "oauth_app.read.any")
	if err != nil {
		return nil, err
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) UpdateClient(ctx context.Context, actor permission.Actor, clientID string, input ClientInput, status string) (map[string]any, error) {
	current, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	if !actor.Has(permission.MustDefinitionByCode("oauth_app.update.any")) {
		status = current.Status
	}
	if status == "" {
		status = current.Status
	}
	if !validClientStatus(status) {
		return nil, badRequest("invalid status")
	}
	client.ID = current.ID
	client.OwnerUserID = current.OwnerUserID
	client.SecretHash = current.SecretHash
	client.Status = status
	client.CreatedAt = current.CreatedAt
	client.UpdatedAt = database.NowMS()
	updated, err := s.DB.OAuth.UpdateClient(ctx, client, permissionIDs)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, notFound("oauth client not found")
	}
	return clientResponse(client, permissionCodes, ""), nil
}

func (s Service) SubmitClientForReview(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	client.Status = StatusPending
	client.UpdatedAt = database.NowMS()
	ok, err := s.DB.OAuth.UpdateClientStatus(ctx, client.ID, client.Status, client.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth client not found")
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) ReviewClient(ctx context.Context, actor permission.Actor, clientID, status string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.update.any")); err != nil {
		return nil, forbidden()
	}
	if !validClientStatus(status) || status == StatusPending {
		return nil, badRequest("invalid status")
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	client.Status = status
	client.UpdatedAt = database.NowMS()
	ok, err := s.DB.OAuth.UpdateClientStatus(ctx, client.ID, status, client.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth client not found")
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) RotateClientSecret(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	if client.ClientType != ClientTypeConfidential {
		return nil, badRequest("public clients do not have secrets")
	}
	raw, hash, err := generateSecret()
	if err != nil {
		return nil, err
	}
	updatedAt := database.NowMS()
	ok, err := s.DB.OAuth.RotateClientSecret(ctx, client.ID, hash, updatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth client not found")
	}
	client.SecretHash = hash
	client.UpdatedAt = updatedAt
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, raw), nil
}

func (s Service) DeleteClient(ctx context.Context, actor permission.Actor, clientID string) error {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.delete.owned", "oauth_app.delete.any")
	if err != nil {
		return err
	}
	owner := client.OwnerUserID
	if actor.Has(permission.MustDefinitionByCode("oauth_app.delete.any")) {
		owner = ""
	}
	ok, err := s.DB.OAuth.DeleteClient(ctx, client.ID, owner)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth client not found")
	}
	return nil
}

func (s Service) ClientPermissions(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("permission.read.any")); err != nil {
		return nil, forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	subjectID := permissiondb.SubjectIDForClient(client.ID)
	effective, err := s.DB.Permissions.EffectivePermissionsForClient(ctx, client.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		return nil, err
	}
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	clientScopes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	overrideItems := make([]map[string]any, 0, len(overrides))
	for _, item := range overrides {
		overrideItems = append(overrideItems, map[string]any{
			"permission_code": item.PermissionCode,
			"effect":          item.Effect,
			"created_at":      item.CreatedAt,
		})
	}
	return map[string]any{
		"subject_id":             subjectID,
		"client":                 publicClient(*client),
		"effective_permissions":  permissionCodesFromBitSet(effective),
		"overrides":              overrideItems,
		"client_allowed_scopes":  clientScopes,
		"session_allowed_scopes": clientCredentialsPolicyCodes(),
	}, nil
}

func (s Service) SetClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code, effect string) error {
	if effect == "allow" {
		if err := actor.Require(permission.MustDefinitionByCode("permission.grant.any")); err != nil {
			return forbidden()
		}
	} else {
		if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
			return forbidden()
		}
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth client not found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok || def.Scope.ID == permission.ScopeSystem {
		return badRequest("invalid permission")
	}
	return s.DB.Permissions.SetPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), def, effect, actor.SubjectID)
}

func (s Service) ClearClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code string) error {
	if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
		return forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth client not found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return badRequest("invalid permission")
	}
	ok, err = s.DB.Permissions.ClearPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), def)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("permission override not found")
	}
	return nil
}

func (s Service) clientFromInput(actor permission.Actor, input ClientInput) (model.OAuthClient, []int64, []string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return model.OAuthClient{}, nil, nil, badRequest("invalid name")
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if !validHTTPURL(redirectURI) {
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
		if !actor.Has(permission.MustDefinitionByCode(code)) {
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
