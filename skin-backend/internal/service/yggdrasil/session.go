package yggdrasil

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
)

const joinSessionTTL = 30 * time.Second

var (
	yggJoinPermission      = permission.MustDefinitionByCode("yggdrasil_server.join.bound_profile")
	yggHasJoinedPermission = permission.MustDefinitionByCode("yggdrasil_server.hasjoined.bound_profile")
)

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
	owned, err := y.DB.Profiles.VerifyOwnership(ctx, t.UserID, profileID)
	if err != nil {
		return err
	}
	if !owned {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	actor, err := y.DB.Permissions.ActorForUser(ctx, t.UserID, permissiondb.EffectiveOptions{
		SessionKind:    permission.SessionKindYggdrasil,
		Entrypoint:     permission.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		return err
	}
	actor.BoundProfileID = profileID
	if !actor.Has(yggJoinPermission) {
		return yggErr(403, "ForbiddenOperationException", "Permission denied.")
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
	if p == nil || p.UserID != t.UserID || p.Name != username {
		return nil, 204, nil
	}
	actor, err := y.DB.Permissions.ActorForUser(ctx, t.UserID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindYggdrasil,
		Entrypoint:  permission.EntrypointYggdrasil,
	})
	if err != nil {
		return nil, 0, err
	}
	actor.BoundProfileID = p.ID
	if !actor.Has(yggHasJoinedPermission) {
		return nil, 0, yggErr(403, "ForbiddenOperationException", "Permission denied.")
	}
	body, err := y.ProfileJSON(*p, true)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}
