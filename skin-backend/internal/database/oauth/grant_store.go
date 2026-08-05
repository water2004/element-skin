package oauth

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) CreateGrant(ctx context.Context, grant model.OAuthGrant, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id, user_id, subject_id, client_id, oidc_scopes, status, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, grant.ID, grant.UserID, grant.SubjectID, grant.ClientID, nonNilStrings(grant.OIDCScopes), grant.Status, grant.CreatedAt, grant.RevokedAt); err != nil {
		return err
	}
	if err := insertGrantPermissions(ctx, tx, grant.ID, permissionIDs, grant.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) RevokeGrant(ctx context.Context, grantID, userID string, revokedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE delegated_permission_grants
		SET status='revoked', revoked_at=$3
		WHERE id=$1 AND ($2='' OR user_id=$2) AND status='active'
	`, grantID, userID, revokedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) RevokeGrantsByClient(ctx context.Context, clientID string, revokedAt int64) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		UPDATE delegated_permission_grants
		SET status='revoked', revoked_at=$2
		WHERE client_id=$1 AND status='active'
		RETURNING id
	`, clientID, revokedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grantIDs []string
	for rows.Next() {
		var grantID string
		if err := rows.Scan(&grantID); err != nil {
			return nil, err
		}
		grantIDs = append(grantIDs, grantID)
	}
	return grantIDs, rows.Err()
}

func (s Store) RevokeInactiveGrants(ctx context.Context, now, createdBefore int64) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE delegated_permission_grants AS g
		SET status='revoked', revoked_at=$1
		WHERE g.status='active'
		  AND g.created_at <= $2
		  AND NOT EXISTS (
			SELECT 1
			FROM oauth_refresh_tokens AS refresh
			WHERE refresh.grant_id=g.id
			  AND refresh.revoked_at IS NULL
			  AND refresh.expires_at>$1
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM oauth_authorization_codes AS code
			WHERE code.grant_id=g.id
			  AND code.expires_at>$1
		  )
	`, now, createdBefore)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) DeleteRevokedGrants(ctx context.Context, cutoff int64) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM delegated_permission_grants
		WHERE status='revoked' AND revoked_at IS NOT NULL AND revoked_at <= $1
	`, cutoff)
	if err != nil {
		return 0, err
	}
	var grantIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		grantIDs = append(grantIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(grantIDs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return 0, nil
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM oauth_authorization_code_permissions
		WHERE code_hash IN (
			SELECT code_hash FROM oauth_authorization_codes WHERE grant_id = ANY($1)
		)
	`, grantIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM oauth_authorization_codes WHERE grant_id = ANY($1)`, grantIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM oauth_refresh_tokens WHERE grant_id = ANY($1)`, grantIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM delegated_grant_permissions WHERE grant_id = ANY($1)`, grantIDs); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
		DELETE FROM delegated_permission_grants
		WHERE id = ANY($1) AND status='revoked' AND revoked_at IS NOT NULL AND revoked_at <= $2
	`, grantIDs, cutoff)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) ListGrantsByUser(ctx context.Context, userID string, limit int) ([]model.OAuthGrant, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, subject_id, client_id, oidc_scopes, status, created_at, revoked_at
		FROM delegated_permission_grants
		WHERE user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []model.OAuthGrant
	for rows.Next() {
		var grant model.OAuthGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.SubjectID, &grant.ClientID, &grant.OIDCScopes, &grant.Status, &grant.CreatedAt, &grant.RevokedAt); err != nil {
			return nil, err
		}
		if len(grant.OIDCScopes) == 0 {
			grant.OIDCScopes = nil
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (s Store) ActiveGrantOIDCScopes(ctx context.Context, grantID, userID, clientID string) ([]string, bool, error) {
	var scopes []string
	err := s.Pool.QueryRow(ctx, `
		SELECT g.oidc_scopes
		FROM delegated_permission_grants g
		JOIN delegated_clients c ON c.id=g.client_id
		WHERE g.id=$1 AND g.user_id=$2 AND g.client_id=$3
		  AND g.status='active' AND c.status='active'
	`, grantID, userID, clientID).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if scopes == nil {
		scopes = []string{}
	}
	return scopes, true, nil
}

func (s Store) GrantPermissionIDs(ctx context.Context, grantID string) ([]int64, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT permission_id
		FROM delegated_grant_permissions
		WHERE grant_id=$1
		ORDER BY permission_id
	`, grantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInt64Rows(rows)
}

func (s Store) ActiveGrantPermissionIDs(ctx context.Context, grantID, userID, clientID string) ([]int64, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT gp.permission_id
		FROM delegated_permission_grants g
		JOIN delegated_clients c ON c.id=g.client_id
		JOIN delegated_grant_permissions gp ON gp.grant_id=g.id
		JOIN delegated_client_permissions cp ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
		WHERE g.id=$1
		  AND g.user_id=$2
		  AND g.client_id=$3
		  AND g.status='active'
		  AND c.status='active'
		ORDER BY gp.permission_id
	`, grantID, userID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInt64Rows(rows)
}
