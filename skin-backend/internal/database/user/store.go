package user

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE email=$1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) Create(ctx context.Context, u model.User) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash)
	return err
}

func (s Store) CreateWithProfile(ctx context.Context, u model.User, p model.Profile, inviteCode, usedBy string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash); err != nil {
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
	for k, v := range fields {
		switch k {
		case "email":
			_, err := s.Pool.Exec(ctx, `UPDATE users SET email=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "display_name":
			_, err := s.Pool.Exec(ctx, `UPDATE users SET display_name=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "preferred_language":
			_, err := s.Pool.Exec(ctx, `UPDATE users SET preferred_language=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "avatar_hash":
			_, err := s.Pool.Exec(ctx, `UPDATE users SET avatar_hash=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		}
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
	for _, q := range []string{
		`DELETE FROM profiles WHERE user_id=$1`,
		`DELETE FROM tokens WHERE user_id=$1`,
		`DELETE FROM site_refresh_tokens WHERE user_id=$1`,
		`DELETE FROM user_textures WHERE user_id=$1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
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
		"banned_until":       u.BannedUntil,
		"preferred_language": u.PreferredLanguage,
		"avatar_hash":        u.AvatarHash,
	}
}

func scan(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Password, &u.IsAdmin, &u.PreferredLanguage, &u.DisplayName, &u.BannedUntil, &u.AvatarHash)
	return u, err
}
