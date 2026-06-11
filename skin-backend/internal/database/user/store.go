package user

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const firstSuperAdminLockID int64 = 0x454C454D454E54
const displayNameLockSeed int64 = 0x444953504C4159

var ErrDisplayNameConflict = errors.New("display name already exists")

type Store struct {
	Pool *pgxpool.Pool
}

func IsEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "users_email_key"
}

func (s Store) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, userSelectSQL+` WHERE email=$1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, userSelectSQL+` WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) Create(ctx context.Context, u model.User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = time.Now().UnixMilli()
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,is_super_admin,display_name,avatar_hash,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.IsSuperAdmin, u.DisplayName, u.AvatarHash, u.CreatedAt)
	return err
}

func (s Store) CreateWithProfile(ctx context.Context, u model.User, p model.Profile, inviteCode, usedBy string) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = time.Now().UnixMilli()
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if u.IsSuperAdmin {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, firstSuperAdminLockID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE is_super_admin=TRUE)`).Scan(&exists); err != nil {
			return err
		}
		if exists {
			u.IsAdmin = false
			u.IsSuperAdmin = false
		}
	}
	if err := lockDisplayName(ctx, tx, u.DisplayName, ""); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,is_super_admin,display_name,avatar_hash,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.IsSuperAdmin, u.DisplayName, u.AvatarHash, u.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash); err != nil {
		return err
	}
	if inviteCode != "" {
		tag, err := tx.Exec(ctx, `UPDATE invites SET used_count=used_count+1 WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)`, inviteCode)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return invite.ErrExhausted
		}
		if usedBy != "" {
			if _, err := tx.Exec(ctx, `UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL`, usedBy, inviteCode); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (s Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s Store) IsDisplayNameTaken(ctx context.Context, name string, exclude string) (bool, error) {
	var one int
	var err error
	if exclude != "" {
		err = s.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1 AND id<>$2`, name, exclude).Scan(&one)
	} else {
		err = s.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1`, name).Scan(&one)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) Update(ctx context.Context, id string, fields map[string]any) error {
	attempted := false
	for _, key := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		if _, ok := fields[key]; ok {
			attempted = true
			break
		}
	}
	if !attempted {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&one); err != nil {
		return err
	}
	if displayName, ok := fields["display_name"].(string); ok {
		if err := lockDisplayName(ctx, tx, displayName, id); err != nil {
			return err
		}
	}
	updated := false
	for _, k := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		v, ok := fields[k]
		if !ok {
			continue
		}
		var tag pgconn.CommandTag
		switch k {
		case "email":
			tag, err = tx.Exec(ctx, `UPDATE users SET email=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "display_name":
			tag, err = tx.Exec(ctx, `UPDATE users SET display_name=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "preferred_language":
			tag, err = tx.Exec(ctx, `UPDATE users SET preferred_language=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "avatar_hash":
			tag, err = tx.Exec(ctx, `UPDATE users SET avatar_hash=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		}
		updated = updated || tag.RowsAffected() > 0
	}
	if !updated {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func lockDisplayName(ctx context.Context, tx pgx.Tx, name, excludeID string) error {
	if name == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,$2))`, name, displayNameLockSeed); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE display_name=$1 AND id<>$2)`,
		name, excludeID,
	).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrDisplayNameConflict
	}
	return nil
}

func (s Store) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	return err
}

func (s Store) UpdatePasswordAndRevokeRefresh(ctx context.Context, id, hash string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) Delete(ctx context.Context, id string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
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
	for _, q := range []string{
		`DELETE FROM profiles WHERE user_id=$1`,
		`DELETE FROM site_refresh_tokens WHERE user_id=$1`,
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
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (s Store) IsBanned(ctx context.Context, id string) (bool, error) {
	var until *int64
	err := s.Pool.QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, id).Scan(&until)
	if errors.Is(err, pgx.ErrNoRows) || until == nil {
		return false, nil
	}
	return *until > time.Now().UnixMilli(), err
}

func PublicUser(u model.User) map[string]any {
	return map[string]any{
		"id":                 u.ID,
		"email":              u.Email,
		"display_name":       u.DisplayName,
		"is_admin":           u.IsAdmin,
		"is_super_admin":     u.IsSuperAdmin,
		"banned_until":       u.BannedUntil,
		"preferred_language": u.PreferredLanguage,
		"avatar_hash":        u.AvatarHash,
		"created_at":         u.CreatedAt,
	}
}

const userSelectSQL = `SELECT id,email,password,is_admin,is_super_admin,preferred_language,display_name,created_at,banned_until,avatar_hash FROM users`

func scan(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Password, &u.IsAdmin, &u.IsSuperAdmin, &u.PreferredLanguage, &u.DisplayName, &u.CreatedAt, &u.BannedUntil, &u.AvatarHash)
	return u, err
}
