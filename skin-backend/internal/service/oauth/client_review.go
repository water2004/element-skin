package oauth

import (
	"context"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
)

func (s Service) ReviewClient(ctx context.Context, actor permission.Actor, clientID, status, reason string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.update.any")); err != nil {
		return nil, forbidden()
	}
	if !validClientStatus(status) || status == StatusPending {
		return nil, badRequest("invalid status")
	}
	reason, err := validateReviewReason(status, reason)
	if err != nil {
		return nil, err
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	if status == StatusActive {
		if err := s.grantReviewedClientPermissions(ctx, actor, client.ID, codes); err != nil {
			return nil, err
		}
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
	if status != StatusActive {
		if err := s.revokeClientAuthorizations(ctx, client.ID, client.UpdatedAt); err != nil {
			return nil, err
		}
	}
	if err := s.notifyOwnerReviewResult(ctx, *client, status, reason); err != nil {
		return nil, err
	}
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, "", endpoints, nil), nil
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
	if err := s.invalidateClientCredentials(ctx, client.ID, updatedAt); err != nil {
		return nil, err
	}
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
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, raw, endpoints, nil), nil
}
