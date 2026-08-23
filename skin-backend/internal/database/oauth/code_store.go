package oauth

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) CreateAuthorizationCode(ctx context.Context, code model.OAuthAuthorizationCode, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := insertAuthorizationCode(ctx, tx, code, permissionIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) UpsertActiveGrantAndCreateAuthorizationCode(ctx context.Context, grant model.OAuthGrant, permissionIDs []int64, code model.OAuthAuthorizationCode) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	grantID, err := upsertActiveGrant(ctx, tx, grant, permissionIDs)
	if err != nil {
		return "", err
	}
	code.GrantID = grantID
	if err := insertAuthorizationCode(ctx, tx, code, permissionIDs); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return grantID, nil
}

func insertAuthorizationCode(ctx context.Context, q queryer, code model.OAuthAuthorizationCode, permissionIDs []int64) error {
	if _, err := q.Exec(ctx, `
		INSERT INTO oauth_authorization_codes
			(code_hash, client_id, user_id, grant_id, redirect_uri, code_challenge,
			 code_challenge_method, oidc_scopes, nonce, expires_at, created_at, consumed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, code.CodeHash, code.ClientID, code.UserID, code.GrantID, code.RedirectURI,
		code.CodeChallenge, code.CodeChallengeMethod, nonNilStrings(code.OIDCScopes), code.Nonce,
		code.ExpiresAt, code.CreatedAt, code.ConsumedAt); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := q.Exec(ctx, `
			INSERT INTO oauth_authorization_code_permissions (code_hash, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, code.CodeHash, permissionID, code.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) ConsumeAuthorizationCode(ctx context.Context, codeHash, clientID, redirectURI string, consumedAt int64) (*model.OAuthAuthorizationCode, []int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		UPDATE oauth_authorization_codes
		SET consumed_at=$4
		WHERE code_hash=$1 AND client_id=$2 AND redirect_uri=$3 AND consumed_at IS NULL AND expires_at>$4
		RETURNING code_hash, client_id, user_id, grant_id, redirect_uri, code_challenge,
		          code_challenge_method, oidc_scopes, nonce, expires_at, created_at, consumed_at
	`, codeHash, clientID, redirectURI, consumedAt)
	code, err := scanAuthorizationCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT permission_id
		FROM oauth_authorization_code_permissions
		WHERE code_hash=$1
		ORDER BY permission_id
	`, codeHash)
	if err != nil {
		return nil, nil, err
	}
	permissionIDs, err := scanInt64Rows(rows)
	if err != nil {
		return nil, nil, err
	}
	return code, permissionIDs, tx.Commit(ctx)
}

func (s Store) CreateRefreshToken(ctx context.Context, refresh model.OAuthToken) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO oauth_refresh_tokens
			(token_hash, client_id, user_id, grant_id, oidc_scopes, expires_at, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, refresh.TokenHash, refresh.ClientID, refresh.UserID, refresh.GrantID,
		nonNilStrings(refresh.OIDCScopes), refresh.ExpiresAt, refresh.CreatedAt, refresh.RevokedAt)
	return err
}

func (s Store) GetRefreshToken(ctx context.Context, tokenHash string) (*model.OAuthToken, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT token_hash, client_id, user_id, grant_id, oidc_scopes, expires_at, created_at, revoked_at
		FROM oauth_refresh_tokens
		WHERE token_hash=$1
	`, tokenHash)
	token, err := scanOAuthToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return token, err
}

func (s Store) RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE oauth_refresh_tokens
		SET revoked_at=$2
		WHERE token_hash=$1 AND revoked_at IS NULL
	`, tokenHash, revokedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) RevokeRefreshTokensByClient(ctx context.Context, clientID string, revokedAt int64) (int64, error) {
	return revokeRefreshTokensByClient(ctx, s.Pool, clientID, revokedAt)
}

func revokeRefreshTokensByClient(ctx context.Context, q queryer, clientID string, revokedAt int64) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE oauth_refresh_tokens
		SET revoked_at=$2
		WHERE client_id=$1 AND revoked_at IS NULL
	`, clientID, revokedAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) RevokeRefreshTokensByGrant(ctx context.Context, grantID string, revokedAt int64) (int64, error) {
	return revokeRefreshTokensByGrant(ctx, s.Pool, grantID, revokedAt)
}

func revokeRefreshTokensByGrant(ctx context.Context, q queryer, grantID string, revokedAt int64) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE oauth_refresh_tokens
		SET revoked_at=$2
		WHERE grant_id=$1 AND revoked_at IS NULL
	`, grantID, revokedAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) DeleteAuthorizationCodesByClient(ctx context.Context, clientID string) (int64, error) {
	return deleteAuthorizationCodesByClient(ctx, s.Pool, clientID)
}

func deleteAuthorizationCodesByClient(ctx context.Context, q queryer, clientID string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM oauth_authorization_codes WHERE client_id=$1`, clientID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) DeleteAuthorizationCodesByGrant(ctx context.Context, grantID string) (int64, error) {
	return deleteAuthorizationCodesByGrant(ctx, s.Pool, grantID)
}

func deleteAuthorizationCodesByGrant(ctx context.Context, q queryer, grantID string) (int64, error) {
	tag, err := q.Exec(ctx, `DELETE FROM oauth_authorization_codes WHERE grant_id=$1`, grantID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) RotateRefreshToken(ctx context.Context, oldRefreshHash string, newRefresh model.OAuthToken, revokedAt int64) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE oauth_refresh_tokens
		SET revoked_at=$2
		WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>$2
	`, oldRefreshHash, revokedAt)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_refresh_tokens
			(token_hash, client_id, user_id, grant_id, oidc_scopes, expires_at, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, newRefresh.TokenHash, newRefresh.ClientID, newRefresh.UserID, newRefresh.GrantID,
		nonNilStrings(newRefresh.OIDCScopes), newRefresh.ExpiresAt, newRefresh.CreatedAt,
		newRefresh.RevokedAt); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
