package user

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s Store) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	var next bool
	err := s.Pool.QueryRow(ctx, `
		UPDATE users
		SET is_admin=CASE WHEN is_super_admin THEN TRUE ELSE NOT is_admin END
		WHERE id=$1
		RETURNING is_admin
	`, id).Scan(&next)
	return next, err
}

func (s Store) TransferSuperAdmin(ctx context.Context, fromID, toID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET is_super_admin=FALSE, is_admin=TRUE WHERE id=$1 AND is_super_admin=TRUE`, fromID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	tag, err = tx.Exec(ctx, `UPDATE users SET is_super_admin=TRUE, is_admin=TRUE WHERE id=$1`, toID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func (s Store) Ban(ctx context.Context, id string, until int64) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s Store) Unban(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
