package site

import (
	"context"
	"errors"
	"strings"

	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

func (s Site) Me(ctx context.Context, userID string) (map[string]any, error) {
	u, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, util.HTTPError{Status: 404, Detail: "user not found"}
	}
	pc, err := s.DB.Profiles.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	tc, err := s.DB.Textures.CountForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": u.ID, "email": u.Email, "lang": u.PreferredLanguage, "display_name": u.DisplayName,
		"is_admin": u.IsAdmin, "is_super_admin": u.IsSuperAdmin, "banned_until": u.BannedUntil, "avatar_hash": u.AvatarHash,
		"profile_count": pc, "texture_count": tc,
	}, nil
}

func (s Site) UpdateMe(ctx context.Context, userID string, body map[string]any) error {
	fields := map[string]any{}
	if v, ok := body["email"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if !validEmail(v) {
			return util.HTTPError{Status: 400, Detail: "Invalid email format"}
		}
		existing, err := s.DB.Users.GetByEmail(ctx, v)
		if err != nil {
			return err
		}
		if existing != nil && existing.ID != userID {
			return util.HTTPError{Status: 400, Detail: "Email already in use"}
		}
		fields["email"] = v
	}
	if v, ok := body["display_name"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			return util.HTTPError{Status: 400, Detail: "Username cannot be empty"}
		}
		if taken, err := s.DB.Users.IsDisplayNameTaken(ctx, v, userID); err != nil {
			return err
		} else if taken {
			return util.HTTPError{Status: 400, Detail: "Username already exists"}
		}
		fields["display_name"] = v
	}
	if v, ok := body["preferred_language"].(string); ok && v != "" {
		fields["preferred_language"] = v
	}
	if v, ok := body["avatar_hash"]; ok {
		fields["avatar_hash"] = v
	}
	if err := s.DB.Users.Update(ctx, userID, fields); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return util.HTTPError{Status: 404, Detail: "user not found"}
		}
		if userstore.IsEmailConflict(err) {
			return util.HTTPError{Status: 400, Detail: "Email already in use"}
		}
		if errors.Is(err, userstore.ErrDisplayNameConflict) {
			return util.HTTPError{Status: 400, Detail: "Username already exists"}
		}
		return err
	}
	return nil
}

func (s Site) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	u, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return util.HTTPError{Status: 404, Detail: "用户不存在"}
	}
	if !util.VerifyPassword(oldPassword, u.Password) {
		return util.HTTPError{Status: 403, Detail: "旧密码错误"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, userID); err != nil {
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, userID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: 404, Detail: "用户不存在"}
	}
	return nil
}
