package oauth

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) CreateClient(ctx context.Context, client model.OAuthClient, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO delegated_clients
			(id, owner_user_id, name, description, redirect_uri, website_url, client_type, secret_hash, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, client.ID, client.OwnerUserID, client.Name, client.Description, client.RedirectURI, client.WebsiteURL, client.ClientType, client.SecretHash, client.Status, client.CreatedAt, client.UpdatedAt); err != nil {
		return err
	}
	if err := insertClientPermissions(ctx, tx, client.ID, permissionIDs, client.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO permission_subjects (id,user_id,kind,status,created_at,updated_at)
		VALUES ($1,NULL,'client','active',$2,$2)
		ON CONFLICT (id) DO UPDATE
		SET kind='client', updated_at=EXCLUDED.updated_at
	`, "client:"+client.ID, client.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) UpdateClient(ctx context.Context, client model.OAuthClient, permissionIDs []int64) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE delegated_clients
		SET name=$2, description=$3, redirect_uri=$4, website_url=$5, client_type=$6, status=$7, updated_at=$8
		WHERE id=$1
	`, client.ID, client.Name, client.Description, client.RedirectURI, client.WebsiteURL, client.ClientType, client.Status, client.UpdatedAt)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM delegated_client_permissions WHERE client_id=$1`, client.ID); err != nil {
		return false, err
	}
	if err := insertClientPermissions(ctx, tx, client.ID, permissionIDs, client.UpdatedAt); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) RotateClientSecret(ctx context.Context, clientID, secretHash string, updatedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE delegated_clients SET secret_hash=$2, updated_at=$3 WHERE id=$1`, clientID, secretHash, updatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) DeleteClient(ctx context.Context, clientID, ownerUserID string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM delegated_clients WHERE id=$1 AND ($2='' OR owner_user_id=$2)`, clientID, ownerUserID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) GetClient(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, owner_user_id, name, description, redirect_uri, website_url, client_type, secret_hash, status, created_at, updated_at
		FROM delegated_clients
		WHERE id=$1
	`, clientID)
	client, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return client, err
}

func (s Store) ListClientsByOwner(ctx context.Context, ownerUserID string, limit int) ([]model.OAuthClient, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, owner_user_id, name, description, redirect_uri, website_url, client_type, secret_hash, status, created_at, updated_at
		FROM delegated_clients
		WHERE owner_user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, ownerUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClients(rows)
}

func (s Store) ListClients(ctx context.Context, limit int) ([]model.OAuthClient, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, owner_user_id, name, description, redirect_uri, website_url, client_type, secret_hash, status, created_at, updated_at
		FROM delegated_clients
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClients(rows)
}

func (s Store) ClientPermissionIDs(ctx context.Context, clientID string) ([]int64, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT permission_id
		FROM delegated_client_permissions
		WHERE client_id=$1
		ORDER BY permission_id
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInt64Rows(rows)
}

func (s Store) CreateGrant(ctx context.Context, grant model.OAuthGrant, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id, user_id, subject_id, client_id, status, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, grant.ID, grant.UserID, grant.SubjectID, grant.ClientID, grant.Status, grant.CreatedAt, grant.RevokedAt); err != nil {
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

func (s Store) ListGrantsByUser(ctx context.Context, userID string, limit int) ([]model.OAuthGrant, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, subject_id, client_id, status, created_at, revoked_at
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
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.SubjectID, &grant.ClientID, &grant.Status, &grant.CreatedAt, &grant.RevokedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
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

func (s Store) CreateAuthorizationCode(ctx context.Context, code model.OAuthAuthorizationCode, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_authorization_codes
			(code_hash, client_id, user_id, grant_id, redirect_uri, code_challenge, code_challenge_method, expires_at, created_at, consumed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, code.CodeHash, code.ClientID, code.UserID, code.GrantID, code.RedirectURI, code.CodeChallenge, code.CodeChallengeMethod, code.ExpiresAt, code.CreatedAt, code.ConsumedAt); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_authorization_code_permissions (code_hash, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, code.CodeHash, permissionID, code.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) ConsumeAuthorizationCode(ctx context.Context, codeHash string, consumedAt int64) (*model.OAuthAuthorizationCode, []int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		UPDATE oauth_authorization_codes
		SET consumed_at=$2
		WHERE code_hash=$1 AND consumed_at IS NULL AND expires_at>$2
		RETURNING code_hash, client_id, user_id, grant_id, redirect_uri, code_challenge, code_challenge_method, expires_at, created_at, consumed_at
	`, codeHash, consumedAt)
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

func (s Store) CreateTokens(ctx context.Context, access model.OAuthToken, refresh model.OAuthToken) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := insertOAuthToken(ctx, tx, "oauth_access_tokens", access); err != nil {
		return err
	}
	if err := insertOAuthToken(ctx, tx, "oauth_refresh_tokens", refresh); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) CreateClientAccessToken(ctx context.Context, token model.OAuthClientAccessToken, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_client_access_tokens (token_hash, client_id, expires_at, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5)
	`, token.TokenHash, token.ClientID, token.ExpiresAt, token.CreatedAt, token.RevokedAt); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_client_access_token_permissions (token_hash, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, token.TokenHash, permissionID, token.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) GetClientAccessToken(ctx context.Context, tokenHash string) (*model.OAuthClientAccessToken, []int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		SELECT token_hash, client_id, expires_at, created_at, revoked_at
		FROM oauth_client_access_tokens
		WHERE token_hash=$1
	`, tokenHash)
	token, err := scanClientAccessToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT permission_id
		FROM oauth_client_access_token_permissions
		WHERE token_hash=$1
		ORDER BY permission_id
	`, tokenHash)
	if err != nil {
		return nil, nil, err
	}
	permissionIDs, err := scanInt64Rows(rows)
	if err != nil {
		return nil, nil, err
	}
	return token, permissionIDs, tx.Commit(ctx)
}

func (s Store) RevokeClientAccessToken(ctx context.Context, tokenHash string, revokedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE oauth_client_access_tokens
		SET revoked_at=$2
		WHERE token_hash=$1 AND revoked_at IS NULL
	`, tokenHash, revokedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) CreateDeviceCode(ctx context.Context, code model.OAuthDeviceCode, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_device_codes
			(device_code_hash, user_code_hash, client_id, user_id, subject_id, status, expires_at, created_at, approved_at, denied_at, consumed_at, last_polled_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, code.DeviceCodeHash, code.UserCodeHash, code.ClientID, code.UserID, code.SubjectID, code.Status, code.ExpiresAt, code.CreatedAt, code.ApprovedAt, code.DeniedAt, code.ConsumedAt, code.LastPolledAt); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_device_code_permissions (device_code_hash, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, code.DeviceCodeHash, permissionID, code.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) GetDeviceCodeByUserCodeHash(ctx context.Context, userCodeHash string) (*model.OAuthDeviceCode, []int64, error) {
	return s.getDeviceCode(ctx, "user_code_hash", userCodeHash)
}

func (s Store) GetDeviceCodeByDeviceCodeHash(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, []int64, error) {
	return s.getDeviceCode(ctx, "device_code_hash", deviceCodeHash)
}

func (s Store) ApproveDeviceCode(ctx context.Context, userCodeHash, userID, subjectID string, approvedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE oauth_device_codes
		SET status='approved', user_id=$2, subject_id=$3, approved_at=$4
		WHERE user_code_hash=$1 AND status='pending' AND expires_at>$4
	`, userCodeHash, userID, subjectID, approvedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) DenyDeviceCode(ctx context.Context, userCodeHash string, deniedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE oauth_device_codes
		SET status='denied', denied_at=$2
		WHERE user_code_hash=$1 AND status='pending' AND expires_at>$2
	`, userCodeHash, deniedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) MarkDeviceCodePolled(ctx context.Context, deviceCodeHash string, polledAt int64) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE oauth_device_codes
		SET last_polled_at=$2
		WHERE device_code_hash=$1
	`, deviceCodeHash, polledAt)
	return err
}

func (s Store) ConsumeApprovedDeviceCode(ctx context.Context, deviceCodeHash string, consumedAt int64) (*model.OAuthDeviceCode, []int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		UPDATE oauth_device_codes
		SET status='consumed', consumed_at=$2
		WHERE device_code_hash=$1 AND status='approved' AND consumed_at IS NULL AND expires_at>$2
		RETURNING device_code_hash, user_code_hash, client_id, user_id, subject_id, status, expires_at, created_at, approved_at, denied_at, consumed_at, last_polled_at
	`, deviceCodeHash, consumedAt)
	code, err := scanDeviceCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	permissions, err := deviceCodePermissionIDs(ctx, tx, deviceCodeHash)
	if err != nil {
		return nil, nil, err
	}
	return code, permissions, tx.Commit(ctx)
}

func (s Store) GetAccessToken(ctx context.Context, tokenHash string) (*model.OAuthToken, error) {
	return s.getToken(ctx, "oauth_access_tokens", tokenHash)
}

func (s Store) GetRefreshToken(ctx context.Context, tokenHash string) (*model.OAuthToken, error) {
	return s.getToken(ctx, "oauth_refresh_tokens", tokenHash)
}

func (s Store) RevokeAccessToken(ctx context.Context, tokenHash string, revokedAt int64) (bool, error) {
	return s.revokeToken(ctx, "oauth_access_tokens", tokenHash, revokedAt)
}

func (s Store) RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt int64) (bool, error) {
	return s.revokeToken(ctx, "oauth_refresh_tokens", tokenHash, revokedAt)
}

func (s Store) RotateRefreshToken(ctx context.Context, oldRefreshHash string, newAccess model.OAuthToken, newRefresh model.OAuthToken, revokedAt int64) (bool, error) {
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
	if err := insertOAuthToken(ctx, tx, "oauth_access_tokens", newAccess); err != nil {
		return false, err
	}
	if err := insertOAuthToken(ctx, tx, "oauth_refresh_tokens", newRefresh); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertClientPermissions(ctx context.Context, q queryer, clientID string, permissionIDs []int64, createdAt int64) error {
	for _, permissionID := range permissionIDs {
		if _, err := q.Exec(ctx, `
			INSERT INTO delegated_client_permissions (client_id, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, clientID, permissionID, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func insertGrantPermissions(ctx context.Context, q queryer, grantID string, permissionIDs []int64, createdAt int64) error {
	for _, permissionID := range permissionIDs {
		if _, err := q.Exec(ctx, `
			INSERT INTO delegated_grant_permissions (grant_id, permission_id, created_at)
			VALUES ($1,$2,$3)
		`, grantID, permissionID, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func scanClient(row rowScanner) (*model.OAuthClient, error) {
	var client model.OAuthClient
	err := row.Scan(&client.ID, &client.OwnerUserID, &client.Name, &client.Description, &client.RedirectURI, &client.WebsiteURL, &client.ClientType, &client.SecretHash, &client.Status, &client.CreatedAt, &client.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func scanClients(rows pgx.Rows) ([]model.OAuthClient, error) {
	var clients []model.OAuthClient
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, *client)
	}
	return clients, rows.Err()
}

func scanAuthorizationCode(row rowScanner) (*model.OAuthAuthorizationCode, error) {
	var code model.OAuthAuthorizationCode
	err := row.Scan(&code.CodeHash, &code.ClientID, &code.UserID, &code.GrantID, &code.RedirectURI, &code.CodeChallenge, &code.CodeChallengeMethod, &code.ExpiresAt, &code.CreatedAt, &code.ConsumedAt)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func scanOAuthToken(row rowScanner) (*model.OAuthToken, error) {
	var token model.OAuthToken
	err := row.Scan(&token.TokenHash, &token.ClientID, &token.UserID, &token.GrantID, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func scanClientAccessToken(row rowScanner) (*model.OAuthClientAccessToken, error) {
	var token model.OAuthClientAccessToken
	err := row.Scan(&token.TokenHash, &token.ClientID, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func scanDeviceCode(row rowScanner) (*model.OAuthDeviceCode, error) {
	var code model.OAuthDeviceCode
	err := row.Scan(&code.DeviceCodeHash, &code.UserCodeHash, &code.ClientID, &code.UserID, &code.SubjectID, &code.Status, &code.ExpiresAt, &code.CreatedAt, &code.ApprovedAt, &code.DeniedAt, &code.ConsumedAt, &code.LastPolledAt)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func scanInt64Rows(rows pgx.Rows) ([]int64, error) {
	defer rows.Close()
	var values []int64
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func insertOAuthToken(ctx context.Context, q queryer, table string, token model.OAuthToken) error {
	_, err := q.Exec(ctx, `
		INSERT INTO `+table+` (token_hash, client_id, user_id, grant_id, expires_at, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, token.TokenHash, token.ClientID, token.UserID, token.GrantID, token.ExpiresAt, token.CreatedAt, token.RevokedAt)
	return err
}

func (s Store) getToken(ctx context.Context, table, tokenHash string) (*model.OAuthToken, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT token_hash, client_id, user_id, grant_id, expires_at, created_at, revoked_at
		FROM `+table+`
		WHERE token_hash=$1
	`, tokenHash)
	token, err := scanOAuthToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return token, err
}

func (s Store) getDeviceCode(ctx context.Context, column, value string) (*model.OAuthDeviceCode, []int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		SELECT device_code_hash, user_code_hash, client_id, user_id, subject_id, status, expires_at, created_at, approved_at, denied_at, consumed_at, last_polled_at
		FROM oauth_device_codes
		WHERE `+column+`=$1
	`, value)
	code, err := scanDeviceCode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	permissions, err := deviceCodePermissionIDs(ctx, tx, code.DeviceCodeHash)
	if err != nil {
		return nil, nil, err
	}
	return code, permissions, tx.Commit(ctx)
}

func deviceCodePermissionIDs(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, deviceCodeHash string) ([]int64, error) {
	rows, err := q.Query(ctx, `
		SELECT permission_id
		FROM oauth_device_code_permissions
		WHERE device_code_hash=$1
		ORDER BY permission_id
	`, deviceCodeHash)
	if err != nil {
		return nil, err
	}
	return scanInt64Rows(rows)
}

func (s Store) revokeToken(ctx context.Context, table, tokenHash string, revokedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE `+table+`
		SET revoked_at=$2
		WHERE token_hash=$1 AND revoked_at IS NULL
	`, tokenHash, revokedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
