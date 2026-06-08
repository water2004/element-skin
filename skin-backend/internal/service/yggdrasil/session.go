package yggdrasil

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

const joinSessionTTL = 30 * time.Second

func (y Yggdrasil) Join(ctx context.Context, access, profileID, serverID, ip string) error {
	t, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err != nil {
		return err
	}
	if t.ProfileID == nil || *t.ProfileID != profileID {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return y.Redis.SetYggSession(ctx, model.Session{ServerID: serverID, AccessToken: access, IP: &ip, CreatedAt: database.NowMS()}, joinSessionTTL)
}

func (y Yggdrasil) HasJoined(ctx context.Context, username, serverID string) (map[string]any, int, error) {
	s, err := y.Redis.GetYggSession(ctx, serverID)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, 204, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if database.NowMS()-s.CreatedAt > int64(joinSessionTTL/time.Millisecond) {
		return nil, 204, nil
	}
	t, err := y.Redis.GetYggToken(ctx, s.AccessToken)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, 204, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if t.ProfileID == nil {
		return nil, 204, nil
	}
	p, err := y.DB.Profiles.GetByID(ctx, *t.ProfileID)
	if err != nil {
		return nil, 0, err
	}
	if p == nil || p.Name != username {
		return nil, 204, nil
	}
	if banned, err := y.DB.Users.IsBanned(ctx, p.UserID); err != nil {
		return nil, 0, err
	} else if banned {
		return nil, 0, yggErr(403, "ForbiddenOperationException", "Account is banned. Please contact administrator.")
	}
	body, err := y.ProfileJSON(*p, true)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}
