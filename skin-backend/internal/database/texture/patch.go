package texture

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Patch struct {
	Note     *string
	Model    *string
	IsPublic *bool
}

func (s Store) UpdateForUser(ctx context.Context, userID, hash, textureType string, patch Patch) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE user_textures SET
			note=CASE WHEN $1 THEN $2 ELSE note END,
			model=CASE WHEN $3 THEN $4 ELSE model END,
			is_public=CASE WHEN $5 AND is_public != 2 THEN $6 ELSE is_public END
		WHERE user_id=$7 AND hash=$8 AND texture_type=$9
	`, patch.Note != nil, stringValue(patch.Note), patch.Model != nil, stringValue(patch.Model),
		patch.IsPublic != nil, publicValue(patch.IsPublic), userID, hash, textureType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if patch.Note != nil {
		if _, err := tx.Exec(ctx, `UPDATE skin_library SET name=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`,
			*patch.Note, hash, userID, textureType); err != nil {
			return err
		}
	}
	if patch.Model != nil {
		if _, err := tx.Exec(ctx, `UPDATE skin_library SET model=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`,
			*patch.Model, hash, userID, textureType); err != nil {
			return err
		}
		if textureType == "skin" {
			if _, err := tx.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE skin_hash=$2 AND user_id=$3`,
				*patch.Model, hash, userID); err != nil {
				return err
			}
		}
	}
	if patch.IsPublic != nil {
		if _, err := tx.Exec(ctx, `UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`,
			publicValue(patch.IsPublic), hash, userID, textureType); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) AdminPatch(ctx context.Context, hash, textureType string, patch Patch) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM skin_library WHERE skin_hash=$1 AND texture_type=$2`, hash, textureType).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE skin_library SET
			name=CASE WHEN $1 THEN $2 ELSE name END,
			model=CASE WHEN $3 THEN $4 ELSE model END,
			is_public=CASE WHEN $5 THEN $6 ELSE is_public END
		WHERE skin_hash=$7 AND texture_type=$8
	`, patch.Note != nil, stringValue(patch.Note), patch.Model != nil, stringValue(patch.Model),
		patch.IsPublic != nil, publicValue(patch.IsPublic), hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE user_textures SET
			note=CASE WHEN $1 THEN $2 ELSE note END,
			model=CASE WHEN $3 THEN $4 ELSE model END,
			is_public=CASE WHEN $5 AND is_public != 2 THEN $6 ELSE is_public END
		WHERE hash=$7 AND texture_type=$8
	`, patch.Note != nil, stringValue(patch.Note), patch.Model != nil, stringValue(patch.Model),
		patch.IsPublic != nil, publicValue(patch.IsPublic), hash, textureType); err != nil {
		return err
	}
	if patch.Model != nil && textureType == "skin" {
		if _, err := tx.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE skin_hash=$2`, *patch.Model, hash); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func publicValue(value *bool) int {
	if value != nil && *value {
		return 1
	}
	return 0
}
