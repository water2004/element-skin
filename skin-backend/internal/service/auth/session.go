package auth

import (
	"context"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type pendingSession struct {
	response    map[string]any
	refreshHash string
	expiresAt   int64
	createdAt   int64
}

func (s Service) prepareSession(ctx context.Context, userID string, extra map[string]any) (pendingSession, error) {
	expireDays, err := s.settings().Int(ctx, "jwt_expire_days", s.Cfg.JWTExpireDays)
	if err != nil {
		return pendingSession{}, err
	}
	access, err := util.CreateAccessToken(s.Cfg.JWTSecret, userID, time.Duration(s.Cfg.AccessMinutes)*time.Minute)
	if err != nil {
		return pendingSession{}, err
	}
	actor, err := s.DB.Permissions.ActorForUser(ctx, userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		return pendingSession{}, err
	}
	rawRefresh, refreshHash, err := util.GenerateRefreshToken()
	if err != nil {
		return pendingSession{}, err
	}
	now := database.NowMS()
	out := map[string]any{
		"access_token":            access,
		"refresh_token":           rawRefresh,
		"permissions":             actor.PermissionCodes(),
		"refresh_max_age_seconds": expireDays * 24 * 3600,
	}
	for k, v := range extra {
		out[k] = v
	}
	return pendingSession{
		response:    out,
		refreshHash: refreshHash,
		expiresAt:   now + int64(expireDays)*24*3600*1000,
		createdAt:   now,
	}, nil
}

func (s Service) issueSession(ctx context.Context, userID string, extra map[string]any) (map[string]any, error) {
	pending, err := s.prepareSession(ctx, userID, extra)
	if err != nil {
		return nil, err
	}
	if err := s.DB.Tokens.AddRefresh(ctx, pending.refreshHash, userID, pending.expiresAt, pending.createdAt); err != nil {
		return nil, err
	}
	return pending.response, nil
}

func (s Service) RotateRefresh(ctx context.Context, raw string) (map[string]any, error) {
	oldHash := util.HashRefreshToken(raw)
	row, err := s.DB.Tokens.GetRefresh(ctx, oldHash)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, util.HTTPError{Status: 401, Object: "refresh_token", Operation: "verify", Reason: "invalid"}
	}
	if database.NowMS() >= row["expires_at"].(int64) {
		if err := s.DB.Tokens.DeleteRefresh(ctx, oldHash); err != nil {
			return nil, err
		}
		return nil, util.HTTPError{Status: 401, Object: "refresh_token", Operation: "verify", Reason: "expired"}
	}
	user, err := s.DB.Users.GetByID(ctx, row["user_id"].(string))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: 401, Object: "refresh_token", Operation: "verify", Reason: "invalid"}
	}
	pending, err := s.prepareSession(ctx, user.ID, nil)
	if err != nil {
		return nil, err
	}
	rotated, err := s.DB.Tokens.RotateRefresh(ctx, oldHash, pending.refreshHash, user.ID, pending.expiresAt, pending.createdAt)
	if err != nil {
		return nil, err
	}
	if !rotated {
		return nil, util.HTTPError{Status: 401, Object: "refresh_token", Operation: "verify", Reason: "invalid"}
	}
	return pending.response, nil
}

func (s Service) RevokeRefresh(ctx context.Context, raw string) error {
	return s.DB.Tokens.DeleteRefresh(ctx, util.HashRefreshToken(raw))
}
