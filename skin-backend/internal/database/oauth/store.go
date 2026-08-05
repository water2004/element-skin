package oauth

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
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
	err := row.Scan(&code.CodeHash, &code.ClientID, &code.UserID, &code.GrantID, &code.RedirectURI, &code.CodeChallenge, &code.CodeChallengeMethod, &code.OIDCScopes, &code.Nonce, &code.ExpiresAt, &code.CreatedAt, &code.ConsumedAt)
	if err != nil {
		return nil, err
	}
	if len(code.OIDCScopes) == 0 {
		code.OIDCScopes = nil
	}
	return &code, nil
}

func scanOAuthToken(row rowScanner) (*model.OAuthToken, error) {
	var token model.OAuthToken
	err := row.Scan(&token.TokenHash, &token.ClientID, &token.UserID, &token.GrantID, &token.OIDCScopes, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt)
	if err != nil {
		return nil, err
	}
	if len(token.OIDCScopes) == 0 {
		token.OIDCScopes = nil
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
