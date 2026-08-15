package oauth

import (
	"context"
)

type RevokedGrantDependency struct {
	GrantID    string
	UserID     string
	ClientID   string
	ClientName string
	RevokedAt  int64
}

type DisabledClientDependency struct {
	ClientID    string
	OwnerUserID string
	Name        string
	UpdatedAt   int64
}

func (s Store) RevokeInvalidGrantsForUser(ctx context.Context, userID string, allowedPermissionIDs []int64, revokedAt int64) ([]RevokedGrantDependency, error) {
	return revokeInvalidGrantsForUser(ctx, s.Pool, userID, allowedPermissionIDs, revokedAt)
}

func revokeInvalidGrantsForUser(ctx context.Context, q transactionQueryer, userID string, allowedPermissionIDs []int64, revokedAt int64) ([]RevokedGrantDependency, error) {
	rows, err := q.Query(ctx, `
		WITH invalid AS (
			SELECT DISTINCT g.id, c.name AS client_name
			FROM delegated_permission_grants g
			JOIN delegated_clients c ON c.id = g.client_id
			WHERE g.user_id=$1
			  AND g.status='active'
			  AND EXISTS (
			      SELECT 1
			      FROM delegated_grant_permissions gp
			      LEFT JOIN delegated_client_permissions cp
			        ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
			      WHERE gp.grant_id=g.id
			        AND (NOT (gp.permission_id = ANY($2::bigint[])) OR cp.permission_id IS NULL)
			  )
		),
		updated AS (
			UPDATE delegated_permission_grants g
			SET status='revoked', revoked_at=$3
			FROM invalid
			WHERE g.id = invalid.id
			RETURNING g.id, g.user_id, g.client_id, invalid.client_name, g.revoked_at
		)
		SELECT id, user_id, client_id, client_name, revoked_at
		FROM updated
		ORDER BY client_name, id
	`, userID, allowedPermissionIDs, revokedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []RevokedGrantDependency{}
	for rows.Next() {
		var item RevokedGrantDependency
		if err := rows.Scan(&item.GrantID, &item.UserID, &item.ClientID, &item.ClientName, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) RevokeInvalidGrantsForClient(ctx context.Context, clientID string, revokedAt int64) ([]RevokedGrantDependency, error) {
	return revokeInvalidGrantsForClient(ctx, s.Pool, clientID, revokedAt)
}

func revokeInvalidGrantsForClient(ctx context.Context, q transactionQueryer, clientID string, revokedAt int64) ([]RevokedGrantDependency, error) {
	rows, err := q.Query(ctx, `
		WITH invalid AS (
			SELECT DISTINCT g.id, c.name AS client_name
			FROM delegated_permission_grants g
			JOIN delegated_clients c ON c.id=g.client_id
			WHERE g.client_id=$1
			  AND g.status='active'
			  AND EXISTS (
			      SELECT 1
			      FROM delegated_grant_permissions gp
			      LEFT JOIN delegated_client_permissions cp
			        ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
			      WHERE gp.grant_id=g.id AND cp.permission_id IS NULL
			  )
		),
		updated AS (
			UPDATE delegated_permission_grants g
			SET status='revoked', revoked_at=$2
			FROM invalid
			WHERE g.id=invalid.id
			RETURNING g.id, g.user_id, g.client_id, invalid.client_name, g.revoked_at
		)
		SELECT id, user_id, client_id, client_name, revoked_at
		FROM updated
		ORDER BY user_id, id
	`, clientID, revokedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RevokedGrantDependency{}
	for rows.Next() {
		var item RevokedGrantDependency
		if err := rows.Scan(&item.GrantID, &item.UserID, &item.ClientID, &item.ClientName, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) DisableInvalidClientsForOwner(ctx context.Context, ownerUserID string, allowedPermissionIDs []int64, exemptPermissionIDs []int64, updatedAt int64) ([]DisabledClientDependency, error) {
	return disableInvalidClientsForOwner(ctx, s.Pool, ownerUserID, allowedPermissionIDs, exemptPermissionIDs, updatedAt)
}

func disableInvalidClientsForOwner(ctx context.Context, q transactionQueryer, ownerUserID string, allowedPermissionIDs []int64, exemptPermissionIDs []int64, updatedAt int64) ([]DisabledClientDependency, error) {
	rows, err := q.Query(ctx, `
		WITH invalid AS (
			SELECT DISTINCT c.id
			FROM delegated_clients c
			WHERE c.owner_user_id=$1
			  AND c.status IN ('pending', 'active')
			  AND EXISTS (
			      SELECT 1
			      FROM delegated_client_permissions cp
			      WHERE cp.client_id=c.id
			        AND NOT (
			            cp.permission_id = ANY($2::bigint[])
			            OR cp.permission_id = ANY($3::bigint[])
			        )
			  )
		),
		updated AS (
			UPDATE delegated_clients c
			SET status='disabled', updated_at=$4
			FROM invalid
			WHERE c.id = invalid.id
			RETURNING c.id, c.owner_user_id, c.name, c.updated_at
		)
		SELECT id, owner_user_id, name, updated_at
		FROM updated
		ORDER BY name, id
	`, ownerUserID, allowedPermissionIDs, exemptPermissionIDs, updatedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []DisabledClientDependency{}
	for rows.Next() {
		var item DisabledClientDependency
		if err := rows.Scan(&item.ClientID, &item.OwnerUserID, &item.Name, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) RevokeInvalidGrantsForUserWithCredentials(ctx context.Context, userID string, allowedPermissionIDs []int64, revokedAt int64) ([]RevokedGrantDependency, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	revoked, err := revokeInvalidGrantsForUser(ctx, tx, userID, allowedPermissionIDs, revokedAt)
	if err != nil {
		return nil, err
	}
	for _, item := range revoked {
		if _, err := revokeRefreshTokensByGrant(ctx, tx, item.GrantID, revokedAt); err != nil {
			return nil, err
		}
		if _, err := deleteAuthorizationCodesByGrant(ctx, tx, item.GrantID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return revoked, nil
}

func (s Store) DisableInvalidClientsForOwnerWithCredentials(ctx context.Context, ownerUserID string, allowedPermissionIDs []int64, exemptPermissionIDs []int64, updatedAt int64) ([]DisabledClientDependency, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	disabled, err := disableInvalidClientsForOwner(ctx, tx, ownerUserID, allowedPermissionIDs, exemptPermissionIDs, updatedAt)
	if err != nil {
		return nil, err
	}
	for _, item := range disabled {
		if _, err := applyClientCredentialAction(ctx, tx, item.ClientID, updatedAt, ClientCredentialsRevokeAuthorizations); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return disabled, nil
}
