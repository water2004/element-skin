package user

import (
	"context"
	"errors"
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
	return s.createRegistration(ctx, u, p, inviteCode, usedBy, nil, nil)
}

func (s Store) CreateWithProfileAndIdentity(
	ctx context.Context,
	u model.User,
	p model.Profile,
	inviteCode string,
	usedBy string,
	identity *model.ExternalIdentity,
	credential *model.ExternalIdentityCredential,
) error {
	return s.createRegistration(ctx, u, p, inviteCode, usedBy, identity, credential)
}

func (s Store) createRegistration(
	ctx context.Context,
	u model.User,
	p model.Profile,
	inviteCode string,
	usedBy string,
	identity *model.ExternalIdentity,
	credential *model.ExternalIdentityCredential,
) error {
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
	if identity != nil {
		if credential == nil || identity.ID == "" || credential.IdentityID != identity.ID || identity.UserID != u.ID {
			return errors.New("invalid external identity registration records")
		}
		if credential.GrantedScopes == nil {
			credential.GrantedScopes = []string{}
		}
		if credential.AuthorizationStatus == "" {
			credential.AuthorizationStatus = model.ExternalIdentityAuthorizationActive
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO external_identities (
				id,user_id,provider_id,subject,label,email,email_verified,display_name,
				avatar_url,created_at,updated_at,last_login_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, identity.ID, identity.UserID, identity.ProviderID, identity.Subject, identity.Label,
			identity.Email, identity.EmailVerified, identity.DisplayName, identity.AvatarURL,
			identity.CreatedAt, identity.UpdatedAt, identity.LastLoginAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO external_identity_credentials
				(identity_id,refresh_token_ciphertext,granted_scopes,authorization_status,
				 last_refresh_at,last_refresh_error_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, credential.IdentityID, credential.RefreshTokenCiphertext, credential.GrantedScopes,
			credential.AuthorizationStatus, credential.LastRefreshAt,
			credential.LastRefreshErrorAt, credential.UpdatedAt); err != nil {
			return err
		}
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
