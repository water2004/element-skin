package oauth

import (
	"context"
	"errors"

	webhookdb "element-skin/backend/internal/database/webhook"
	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) CreateClient(ctx context.Context, client model.OAuthClient, permissionIDs []int64, endpoints []model.WebhookEndpoint) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := webhookdb.LockSubscriptionSnapshot(ctx, tx); err != nil {
		return err
	}
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
		INSERT INTO permission_subjects (id,user_id,kind,status,protected,created_at,updated_at)
		VALUES ($1,NULL,'client','active',FALSE,$2,$2)
		ON CONFLICT (id) DO UPDATE
		SET kind='client', updated_at=EXCLUDED.updated_at
	`, "client:"+client.ID, client.CreatedAt); err != nil {
		return err
	}
	if err := webhookdb.ReplaceEndpoints(ctx, tx, client.ID, endpoints); err != nil {
		return err
	}
	if err := webhookdb.RefreshSubscriptionSnapshot(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) UpdateClient(ctx context.Context, client model.OAuthClient, permissionIDs []int64, endpoints []model.WebhookEndpoint) (bool, error) {
	result, err := s.UpdateClientWithCredentials(ctx, client, permissionIDs, endpoints, ClientCredentialsPreserve)
	return result.Updated, err
}

func (s Store) RotateClientSecret(ctx context.Context, clientID, secretHash string, updatedAt int64) (bool, error) {
	return rotateClientSecret(ctx, s.Pool, clientID, secretHash, updatedAt)
}

func rotateClientSecret(ctx context.Context, q queryer, clientID, secretHash string, updatedAt int64) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE delegated_clients SET secret_hash=$2, updated_at=$3 WHERE id=$1`, clientID, secretHash, updatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) DeleteClient(ctx context.Context, clientID, ownerUserID string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := webhookdb.LockSubscriptionSnapshot(ctx, tx); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM delegated_clients WHERE id=$1 AND ($2='' OR owner_user_id=$2)`, clientID, ownerUserID)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM permission_subjects WHERE id=$1`, "client:"+clientID); err != nil {
		return false, err
	}
	if err := webhookdb.RefreshSubscriptionSnapshot(ctx, tx); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
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

func (s Store) ClientIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM delegated_clients WHERE owner_user_id=$1 ORDER BY id`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clientIDs []string
	for rows.Next() {
		var clientID string
		if err := rows.Scan(&clientID); err != nil {
			return nil, err
		}
		clientIDs = append(clientIDs, clientID)
	}
	return clientIDs, rows.Err()
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

func (s Store) ListClientsByStatus(ctx context.Context, status string, limit int) ([]model.OAuthClient, error) {
	if status == "" || status == "all" {
		return s.ListClients(ctx, limit)
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, owner_user_id, name, description, redirect_uri, website_url, client_type, secret_hash, status, created_at, updated_at
		FROM delegated_clients
		WHERE status=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClients(rows)
}

func (s Store) UpdateClientStatus(ctx context.Context, clientID, status string, updatedAt int64) (bool, error) {
	result, err := s.UpdateClientStatusWithCredentials(ctx, clientID, status, updatedAt, ClientCredentialsPreserve)
	return result.Updated, err
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

func (s Store) ClientHasPermission(ctx context.Context, clientID string, permissionID int64) (bool, error) {
	var allowed bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM delegated_clients AS client
			JOIN delegated_client_permissions AS requested ON requested.client_id=client.id
			WHERE client.id=$1 AND client.status='active' AND requested.permission_id=$2
		)
	`, clientID, permissionID).Scan(&allowed)
	return allowed, err
}
