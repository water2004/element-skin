package oauth

import (
	"context"

	"element-skin/backend/internal/database"
	dboauth "element-skin/backend/internal/database/oauth"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func (s Service) ReviewClient(ctx context.Context, actor permission.Actor, clientID, status, reason string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.update.any")); err != nil {
		return nil, forbidden()
	}
	if !validClientStatus(status) || status == StatusPending {
		return nil, badRequest("status", "validate", "invalid")
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
		return nil, notFound("oauth_client", "resolve", "not_found")
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	allowedPermissionIDs := []int64(nil)
	if status == StatusActive {
		allowedPermissionIDs, err = reviewedClientPermissionIDs(codes)
		if err != nil {
			return nil, err
		}
	}
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	client.Status = status
	client.UpdatedAt = database.NowMS()
	action := dboauth.ClientCredentialsPreserve
	if status != StatusActive {
		action = dboauth.ClientCredentialsRevokeAuthorizations
		if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID); err != nil {
			return nil, err
		}
	}
	mutation, err := s.DB.OAuth.ReviewClientWithCredentials(
		ctx,
		client.ID,
		status,
		client.UpdatedAt,
		action,
		allowedPermissionIDs,
		actor.SubjectID,
	)
	if err != nil {
		return nil, err
	}
	if !mutation.Updated {
		return nil, notFound("oauth_client", "resolve", "not_found")
	}
	if status != StatusActive {
		reportPostCommitError("remove reviewed client access-token races", s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID))
	} else if len(allowedPermissionIDs) > 0 {
		reportPostCommitError(
			"invalidate reviewed client permission cache",
			s.DB.Permissions.InvalidateSubjectCache(ctx, permissiondb.SubjectIDForClient(client.ID)),
		)
	}
	reportPostCommitError("notify client owner about review result", s.notifyOwnerReviewResult(ctx, *client, status, reason))
	return clientResponse(*client, codes, "", endpoints, nil), nil
}

func (s Service) RotateClientSecret(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	if client.ClientType != ClientTypeConfidential {
		return nil, badRequest("client_secret", "configure", "unsupported")
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	endpoints, err := s.DB.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	raw, hash, err := generateSecret()
	if err != nil {
		return nil, err
	}
	updatedAt := database.NowMS()
	if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID); err != nil {
		return nil, err
	}
	ok, err := s.DB.OAuth.RotateClientSecretAndCredentials(ctx, client.ID, hash, updatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth_client", "resolve", "not_found")
	}
	client.SecretHash = hash
	client.UpdatedAt = updatedAt
	reportPostCommitError("remove rotated client access-token races", s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID))
	return clientResponse(*client, codes, raw, endpoints, nil), nil
}
