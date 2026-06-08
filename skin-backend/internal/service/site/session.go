package site

import (
	"context"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func (s Site) issueSession(ctx context.Context, userID string, isAdmin bool, extra map[string]any) (map[string]any, error) {
	expireDays, err := s.settings().Int(ctx, "jwt_expire_days", s.Cfg.JWTExpireDays)
	if err != nil {
		return nil, err
	}
	access, err := util.CreateAccessToken(s.Cfg.JWTSecret, userID, isAdmin, time.Duration(s.Cfg.AccessMinutes)*time.Minute)
	if err != nil {
		return nil, err
	}
	rawRefresh, refreshHash, err := util.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	now := database.NowMS()
	if err := s.DB.Tokens.AddRefresh(ctx, refreshHash, userID, now+int64(expireDays)*24*3600*1000, now); err != nil {
		return nil, err
	}
	out := map[string]any{
		"access_token":            access,
		"refresh_token":           rawRefresh,
		"is_admin":                isAdmin,
		"refresh_max_age_seconds": expireDays * 24 * 3600,
	}
	for k, v := range extra {
		out[k] = v
	}
	return out, nil
}

func (s Site) RotateRefresh(ctx context.Context, raw string) (map[string]any, error) {
	row, err := s.DB.Tokens.ConsumeRefresh(ctx, util.HashRefreshToken(raw))
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, util.HTTPError{Status: 401, Detail: "invalid refresh token"}
	}
	if database.NowMS() >= row["expires_at"].(int64) {
		return nil, util.HTTPError{Status: 401, Detail: "refresh token expired"}
	}
	user, err := s.DB.Users.GetByID(ctx, row["user_id"].(string))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: 401, Detail: "invalid refresh token"}
	}
	return s.issueSession(ctx, user.ID, user.IsAdmin, nil)
}

func (s Site) RevokeRefresh(ctx context.Context, raw string) error {
	return s.DB.Tokens.DeleteRefresh(ctx, util.HashRefreshToken(raw))
}
