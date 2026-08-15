package oauth

import (
	"context"
	"errors"

	webhookdb "element-skin/backend/internal/database/webhook"
	"element-skin/backend/internal/model"
)

type ClientCredentialAction uint8

const (
	ClientCredentialsPreserve ClientCredentialAction = iota
	ClientCredentialsInvalidate
	ClientCredentialsRevokeAuthorizations
	ClientCredentialsRevokeInvalidGrants
)

type ClientMutationResult struct {
	Updated       bool
	RevokedGrants []RevokedGrantDependency
}

func (s Store) UpdateClientWithCredentials(
	ctx context.Context,
	client model.OAuthClient,
	permissionIDs []int64,
	endpoints []model.WebhookEndpoint,
	action ClientCredentialAction,
) (ClientMutationResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ClientMutationResult{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE delegated_clients
		SET name=$2, description=$3, redirect_uri=$4, website_url=$5, client_type=$6, status=$7, updated_at=$8
		WHERE id=$1
	`, client.ID, client.Name, client.Description, client.RedirectURI, client.WebsiteURL,
		client.ClientType, client.Status, client.UpdatedAt)
	if err != nil || tag.RowsAffected() == 0 {
		return ClientMutationResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM delegated_client_permissions WHERE client_id=$1`, client.ID); err != nil {
		return ClientMutationResult{}, err
	}
	if err := insertClientPermissions(ctx, tx, client.ID, permissionIDs, client.UpdatedAt); err != nil {
		return ClientMutationResult{}, err
	}
	if err := webhookdb.ReplaceEndpoints(ctx, tx, client.ID, endpoints); err != nil {
		return ClientMutationResult{}, err
	}

	result := ClientMutationResult{Updated: true}
	result.RevokedGrants, err = applyClientCredentialAction(ctx, tx, client.ID, client.UpdatedAt, action)
	if err != nil {
		return ClientMutationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClientMutationResult{}, err
	}
	return result, nil
}

func (s Store) UpdateClientStatusWithCredentials(
	ctx context.Context,
	clientID string,
	status string,
	updatedAt int64,
	action ClientCredentialAction,
) (ClientMutationResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ClientMutationResult{}, err
	}
	defer tx.Rollback(ctx)

	updated, err := updateClientStatus(ctx, tx, clientID, status, updatedAt)
	if err != nil || !updated {
		return ClientMutationResult{}, err
	}
	result := ClientMutationResult{Updated: true}
	result.RevokedGrants, err = applyClientCredentialAction(ctx, tx, clientID, updatedAt, action)
	if err != nil {
		return ClientMutationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClientMutationResult{}, err
	}
	return result, nil
}

func (s Store) ReviewClientWithCredentials(
	ctx context.Context,
	clientID string,
	status string,
	updatedAt int64,
	action ClientCredentialAction,
	allowedPermissionIDs []int64,
	grantedBySubjectID string,
) (ClientMutationResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ClientMutationResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := allowClientPermissions(ctx, tx, clientID, allowedPermissionIDs, grantedBySubjectID, updatedAt); err != nil {
		return ClientMutationResult{}, err
	}
	updated, err := updateClientStatus(ctx, tx, clientID, status, updatedAt)
	if err != nil || !updated {
		return ClientMutationResult{}, err
	}
	result := ClientMutationResult{Updated: true}
	result.RevokedGrants, err = applyClientCredentialAction(ctx, tx, clientID, updatedAt, action)
	if err != nil {
		return ClientMutationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClientMutationResult{}, err
	}
	return result, nil
}

func updateClientStatus(ctx context.Context, q queryer, clientID, status string, updatedAt int64) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE delegated_clients
		SET status=$2, updated_at=$3
		WHERE id=$1
	`, clientID, status, updatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func allowClientPermissions(
	ctx context.Context,
	q queryer,
	clientID string,
	permissionIDs []int64,
	grantedBySubjectID string,
	createdAt int64,
) error {
	var grantedBy any = grantedBySubjectID
	if grantedBySubjectID == "" {
		grantedBy = nil
	}
	for _, permissionID := range permissionIDs {
		if _, err := q.Exec(ctx, `
			INSERT INTO subject_permission_overrides
				(subject_id,permission_id,effect,granted_by_subject_id,created_at)
			VALUES ($1,$2,'allow',$3,$4)
			ON CONFLICT (subject_id,permission_id) DO UPDATE
			SET effect='allow', granted_by_subject_id=EXCLUDED.granted_by_subject_id
		`, "client:"+clientID, permissionID, grantedBy, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) RotateClientSecretAndCredentials(ctx context.Context, clientID, secretHash string, updatedAt int64) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	rotated, err := rotateClientSecret(ctx, tx, clientID, secretHash, updatedAt)
	if err != nil || !rotated {
		return rotated, err
	}
	if _, err := applyClientCredentialAction(ctx, tx, clientID, updatedAt, ClientCredentialsInvalidate); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) RevokeGrantAndCredentials(ctx context.Context, grantID, userID string, revokedAt int64) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	revoked, err := revokeGrant(ctx, tx, grantID, userID, revokedAt)
	if err != nil || !revoked {
		return revoked, err
	}
	if _, err := revokeRefreshTokensByGrant(ctx, tx, grantID, revokedAt); err != nil {
		return false, err
	}
	if _, err := deleteAuthorizationCodesByGrant(ctx, tx, grantID); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func applyClientCredentialAction(
	ctx context.Context,
	q transactionQueryer,
	clientID string,
	revokedAt int64,
	action ClientCredentialAction,
) ([]RevokedGrantDependency, error) {
	var revoked []RevokedGrantDependency
	switch action {
	case ClientCredentialsPreserve:
		return nil, nil
	case ClientCredentialsInvalidate:
	case ClientCredentialsRevokeAuthorizations:
		if _, err := revokeGrantsByClient(ctx, q, clientID, revokedAt); err != nil {
			return nil, err
		}
	case ClientCredentialsRevokeInvalidGrants:
		var err error
		revoked, err = revokeInvalidGrantsForClient(ctx, q, clientID, revokedAt)
		if err != nil {
			return nil, err
		}
		for _, item := range revoked {
			if _, err := revokeRefreshTokensByGrant(ctx, q, item.GrantID, revokedAt); err != nil {
				return nil, err
			}
			if _, err := deleteAuthorizationCodesByGrant(ctx, q, item.GrantID); err != nil {
				return nil, err
			}
		}
		return revoked, nil
	default:
		return nil, errors.New("invalid oauth client credential action")
	}
	if _, err := revokeRefreshTokensByClient(ctx, q, clientID, revokedAt); err != nil {
		return nil, err
	}
	if _, err := deleteAuthorizationCodesByClient(ctx, q, clientID); err != nil {
		return nil, err
	}
	return revoked, nil
}
