package user

import (
	"context"
	"time"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"
)

func (s Store) Create(ctx context.Context, u model.User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = time.Now().UnixMilli()
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id,email,password,display_name,avatar_hash,created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.DisplayName, u.AvatarHash, u.CreatedAt)
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
	if err := lockDisplayName(ctx, tx, u.DisplayName, ""); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password,display_name,avatar_hash,created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.DisplayName, u.AvatarHash, u.CreatedAt); err != nil {
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
