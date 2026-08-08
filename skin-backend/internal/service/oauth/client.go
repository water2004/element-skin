package oauth

import (
	"context"
	"strings"

	"element-skin/backend/internal/database"
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
	endpoints, endpointSecrets, err := s.prepareWebhookEndpoints(ctx, client.ID, input.WebhookEndpoints, permissionCodes, client.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := s.DB.OAuth.CreateClientWithEndpoints(ctx, client, permissionIDs, endpoints); err != nil {
		return nil, err
	}
	if err := s.notifyAdminsClientSubmitted(ctx, client); err != nil {
		return nil, err
	}
	return clientResponse(client, permissionCodes, secret, endpoints, endpointSecrets), nil
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
		endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, clientResponse(client, codes, "", endpoints, nil))
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
		out = append(out, adminClientSummary(client))
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
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, "", endpoints, nil), nil
}

func (s Service) UpdateClient(ctx context.Context, actor permission.Actor, clientID string, input ClientInput, status string) (map[string]any, error) {
	current, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	currentPermissionCodes, err := s.clientPermissionCodes(ctx, current.ID)
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
	endpoints, endpointSecrets, err := s.prepareWebhookEndpoints(ctx, client.ID, input.WebhookEndpoints, permissionCodes, client.UpdatedAt)
	if err != nil {
		return nil, err
	}
	updated, err := s.DB.OAuth.UpdateClientWithEndpoints(ctx, client, permissionIDs, endpoints)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, notFound("oauth client not found")
	}
	if client.Status != StatusActive {
		if err := s.revokeClientAuthorizations(ctx, client.ID, client.UpdatedAt); err != nil {
			return nil, err
		}
	} else if hasRemovedPermission(currentPermissionCodes, permissionCodes) {
		revoked, err := s.DB.OAuth.RevokeInvalidGrantsForClient(ctx, client.ID, client.UpdatedAt)
		if err != nil {
			return nil, err
		}
		for _, item := range revoked {
			if err := s.invalidateGrantCredentials(ctx, item.GrantID, client.UpdatedAt); err != nil {
				return nil, err
			}
		}
		if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID); err != nil {
			return nil, err
		}
		if err := s.notifyPermissionDependencyChanges(ctx, PermissionDependencyResult{RevokedGrants: revoked}); err != nil {
			return nil, err
		}
	} else if current.RedirectURI != client.RedirectURI || current.ClientType != client.ClientType {
		if err := s.invalidateClientCredentials(ctx, client.ID, client.UpdatedAt); err != nil {
			return nil, err
		}
	}
	return clientResponse(client, permissionCodes, "", endpoints, endpointSecrets), nil
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
	if err := s.revokeClientAuthorizations(ctx, client.ID, client.UpdatedAt); err != nil {
		return nil, err
	}
	if err := s.notifyAdminsClientSubmitted(ctx, *client); err != nil {
		return nil, err
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, "", endpoints, nil), nil
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
	return s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID)
}

func hasRemovedPermission(before, after []string) bool {
	afterSet := make(map[string]struct{}, len(after))
	for _, code := range after {
		afterSet[code] = struct{}{}
	}
	for _, code := range before {
		if _, ok := afterSet[code]; !ok {
			return true
		}
	}
	return false
}
