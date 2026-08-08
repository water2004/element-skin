package oauth

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

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
	code, permissions, err := consumeApprovedDeviceCode(ctx, tx, deviceCodeHash, consumedAt)
	if err != nil || code == nil {
		return code, permissions, err
	}
	return code, permissions, tx.Commit(ctx)
}

func (s Store) ConsumeApprovedDeviceCodeAndUpsertGrant(ctx context.Context, deviceCodeHash string, consumedAt int64, grant model.OAuthGrant) (*model.OAuthDeviceCode, []int64, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	defer tx.Rollback(ctx)
	code, permissionIDs, err := consumeApprovedDeviceCode(ctx, tx, deviceCodeHash, consumedAt)
	if err != nil || code == nil {
		return code, permissionIDs, "", err
	}
	if code.UserID == nil || code.SubjectID == nil {
		return code, permissionIDs, "", nil
	}
	grant.UserID = *code.UserID
	grant.SubjectID = *code.SubjectID
	grant.ClientID = code.ClientID
	grantID, err := upsertActiveGrant(ctx, tx, grant, permissionIDs)
	if err != nil {
		return nil, nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, "", err
	}
	return code, permissionIDs, grantID, nil
}

func consumeApprovedDeviceCode(ctx context.Context, q transactionQueryer, deviceCodeHash string, consumedAt int64) (*model.OAuthDeviceCode, []int64, error) {
	row := q.QueryRow(ctx, `
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
	permissions, err := deviceCodePermissionIDs(ctx, q, deviceCodeHash)
	if err != nil {
		return nil, nil, err
	}
	return code, permissions, nil
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
