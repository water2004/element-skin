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
	"element-skin/backend/internal/util"
)

const tokenTTL = 15 * 24 * time.Hour

var (
	yggSessionCreatePermission     = permission.MustDefinitionByCode("yggdrasil_session.create.owned")
	yggSessionRefreshPermission    = permission.MustDefinitionByCode("yggdrasil_session.refresh.owned")
	yggSessionValidatePermission   = permission.MustDefinitionByCode("yggdrasil_session.validate.owned")
	yggSessionInvalidatePermission = permission.MustDefinitionByCode("yggdrasil_session.invalidate.owned")
	yggSessionSignoutPermission    = permission.MustDefinitionByCode("yggdrasil_session.signout.owned")
)

func (y Yggdrasil) Authenticate(ctx context.Context, username, password, clientToken string, requestUser bool) (map[string]any, error) {
	u, loginProfile, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	if err := y.requireYggPermission(ctx, u.ID, yggSessionCreatePermission); err != nil {
		return nil, err
	}
	access, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if clientToken == "" {
		clientToken = access
	}
	profiles, err := y.DB.Profiles.GetByUser(ctx, u.ID, 100)
	if err != nil {
		return nil, err
	}
	var selected *model.Profile
	if loginProfile != nil {
		selected = loginProfile
	} else if len(profiles) == 1 {
		selected = &profiles[0]
	}
	var pid *string
	if selected != nil {
		pid = &selected.ID
	}
	createdAt := database.NowMS()
	if err := y.Redis.SetYggToken(ctx, model.Token{AccessToken: access, ClientToken: clientToken, UserID: u.ID, ProfileID: pid, CreatedAt: createdAt}, tokenTTL); err != nil {
		return nil, err
	}
	if err := y.Redis.TrimYggTokensByUser(ctx, u.ID, 5); err != nil {
		_ = y.Redis.DeleteYggToken(ctx, access)
		return nil, err
	}
	available := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		available = append(available, map[string]any{"id": p.ID, "name": p.Name})
	}
	resp := map[string]any{"accessToken": access, "clientToken": clientToken, "availableProfiles": available}
	if selected != nil {
		resp["selectedProfile"] = map[string]any{"id": selected.ID, "name": selected.Name}
	}
	if requestUser {
		resp["user"] = yggUserPayload(*u)
	}
	return resp, nil
}

func (y Yggdrasil) verifyCredentials(ctx context.Context, username, password string) (*model.User, *model.Profile, error) {
	u, err := y.DB.Users.GetByEmail(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	var p *model.Profile
	if u == nil {
		p, err = y.DB.Profiles.GetByName(ctx, username)
		if err != nil {
			return nil, nil, err
		}
		if p != nil {
			u, err = y.DB.Users.GetByID(ctx, p.UserID)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if u == nil || !util.VerifyPassword(password, u.Password) {
		return nil, nil, nil
	}
	return u, p, nil
}

func (y Yggdrasil) Refresh(ctx context.Context, accessToken, clientToken, selectedID string, requestUser bool) (map[string]any, error) {
	t, err := y.Redis.GetYggToken(ctx, accessToken)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err != nil {
		return nil, err
	}
	if clientToken != "" && clientToken != t.ClientToken {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionRefreshPermission); err != nil {
		return nil, err
	}
	if ok, err := y.tokenProfileOwned(ctx, t); err != nil {
		return nil, err
	} else if !ok {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	newProfile := t.ProfileID
	var selected map[string]any
	if selectedID != "" {
		selectedID = util.StripUUIDDashes(selectedID)
		if t.ProfileID != nil {
			return nil, yggErr(400, "IllegalArgumentException", "Access token already has a profile assigned.")
		}
		ok, err := y.DB.Profiles.VerifyOwnership(ctx, t.UserID, selectedID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid profile.")
		}
		newProfile = &selectedID
	}
	if newProfile != nil {
		p, err := y.DB.Profiles.GetByID(ctx, *newProfile)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
		}
		selected = map[string]any{"id": p.ID, "name": p.Name}
	}
	var responseUser *model.User
	if requestUser {
		responseUser, err = y.DB.Users.GetByID(ctx, t.UserID)
		if err != nil {
			return nil, err
		}
		if responseUser == nil {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
		}
	}
	newAccess, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	createdAt := database.NowMS()
	replaced, err := y.Redis.ReplaceYggToken(ctx, accessToken, model.Token{AccessToken: newAccess, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: newProfile, CreatedAt: createdAt}, tokenTTL)
	if err != nil {
		return nil, err
	}
	if !replaced {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	resp := map[string]any{"accessToken": newAccess, "clientToken": t.ClientToken}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	if responseUser != nil {
		resp["user"] = yggUserPayload(*responseUser)
	}
	return resp, nil
}

func (y Yggdrasil) Validate(ctx context.Context, access, client string) error {
	t, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err != nil {
		return err
	}
	if (client != "" && client != t.ClientToken) || database.NowMS()-t.CreatedAt > int64(tokenTTL/time.Millisecond) {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionValidatePermission); err != nil {
		return err
	}
	if ok, err := y.tokenProfileOwned(ctx, t); err != nil {
		return err
	} else if !ok {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return nil
}

func (y Yggdrasil) Token(ctx context.Context, access string) (model.Token, error) {
	token, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return model.Token{}, yggErr(401, "Unauthorized", "Invalid token")
	}
	if err != nil {
		return model.Token{}, err
	}
	if ok, err := y.tokenProfileOwned(ctx, token); err != nil {
		return model.Token{}, err
	} else if !ok {
		return model.Token{}, yggErr(401, "Unauthorized", "Invalid token")
	}
	return token, nil
}

func (y Yggdrasil) tokenProfileOwned(ctx context.Context, token model.Token) (bool, error) {
	if token.ProfileID == nil {
		return true, nil
	}
	return y.DB.Profiles.VerifyOwnership(ctx, token.UserID, *token.ProfileID)
}

func (y Yggdrasil) Invalidate(ctx context.Context, access string) error {
	if access == "" {
		return nil
	}
	t, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionInvalidatePermission); err != nil {
		return err
	}
	return y.Redis.DeleteYggToken(ctx, access)
}

func (y Yggdrasil) Signout(ctx context.Context, username, password string) error {
	u, _, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return err
	}
	if u == nil {
		return yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	if err := y.requireYggPermission(ctx, u.ID, yggSessionSignoutPermission); err != nil {
		return err
	}
	return y.Redis.DeleteYggTokensByUser(ctx, u.ID)
}

func yggUserPayload(u model.User) map[string]any {
	return map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
}

func (y Yggdrasil) requireYggPermission(ctx context.Context, userID string, def permission.Definition) error {
	actor, err := y.DB.Permissions.ActorForUser(ctx, userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindYggdrasil,
		Entrypoint:  permission.EntrypointYggdrasil,
	})
	if err != nil {
		return err
	}
	if !actor.Has(def) {
		return yggErr(403, "ForbiddenOperationException", "Permission denied.")
	}
	return nil
}
