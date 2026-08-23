package user

import (
	"context"
	"errors"

	webhookdb "element-skin/backend/internal/database/webhook"

	"github.com/jackc/pgx/v5"
)

func (s Store) Delete(ctx context.Context, id string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := webhookdb.LockSubscriptionSnapshot(ctx, tx); err != nil {
		return false, err
	}
	type textureKey struct {
		hash        string
		textureType string
	}
	rows, err := tx.Query(ctx, `
		SELECT sl.skin_hash, sl.texture_type
		FROM skin_library sl
		WHERE sl.uploader=$1
		   OR EXISTS (
				SELECT 1
				FROM user_textures ut
				WHERE ut.user_id=$1
				  AND ut.hash=sl.skin_hash
				  AND ut.texture_type=sl.texture_type
		   )
		ORDER BY sl.skin_hash, sl.texture_type
		FOR UPDATE
	`, id)
	if err != nil {
		return false, err
	}
	var affectedTextures []textureKey
	for rows.Next() {
		var key textureKey
		if err := rows.Scan(&key.hash, &key.textureType); err != nil {
			rows.Close()
			return false, err
		}
		affectedTextures = append(affectedTextures, key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	clientRows, err := tx.Query(ctx, `SELECT id FROM delegated_clients WHERE owner_user_id=$1 ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return false, err
	}
	var clientSubjectIDs []string
	for clientRows.Next() {
		var clientID string
		if err := clientRows.Scan(&clientID); err != nil {
			clientRows.Close()
			return false, err
		}
		clientSubjectIDs = append(clientSubjectIDs, "client:"+clientID)
	}
	if err := clientRows.Err(); err != nil {
		clientRows.Close()
		return false, err
	}
	clientRows.Close()
	for _, q := range []string{
		`DELETE FROM profiles WHERE user_id=$1`,
		`DELETE FROM site_refresh_tokens WHERE user_id=$1`,
		`DELETE FROM oauth_device_codes WHERE user_id=$1`,
		`DELETE FROM user_textures
		 WHERE (hash,texture_type) IN (
			SELECT skin_hash,texture_type FROM skin_library WHERE uploader=$1
		 )`,
		`DELETE FROM skin_library WHERE uploader=$1`,
		`DELETE FROM user_textures WHERE user_id=$1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return false, err
		}
	}
	for _, key := range affectedTextures {
		if _, err := tx.Exec(ctx, `
			UPDATE skin_library
			SET usage_count=(
				SELECT COUNT(*)
				FROM user_textures
				WHERE hash=$1 AND texture_type=$2
			)
			WHERE skin_hash=$1 AND texture_type=$2
		`, key.hash, key.textureType); err != nil {
			return false, err
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	if len(clientSubjectIDs) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM permission_subjects WHERE id = ANY($1::text[])`, clientSubjectIDs); err != nil {
			return false, err
		}
	}
	if err := webhookdb.RefreshSubscriptionSnapshot(ctx, tx); err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}
