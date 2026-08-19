package account

import (
	"context"
	"errors"
	"net/http"
	"strings"

	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/permission"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

var (
	accountReadSelfPermission    = permission.MustDefinitionByCode("account.read.self")
	accountUpdateSelfPermission  = permission.MustDefinitionByCode("account.update.self")
	accountDeleteSelfPermission  = permission.MustDefinitionByCode("account.delete.self")
	passwordUpdateSelfPermission = permission.MustDefinitionByCode("account_password.update.self")
)

func (s AccountService) Me(ctx context.Context, actor permission.Actor) (map[string]any, error) {
	if err := actor.Require(accountReadSelfPermission); err != nil {
		return nil, permissionDenied()
	}
	u, err := s.DB.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	profileCount, err := s.DB.Profiles.CountByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	textureCount, err := s.DB.Textures.CountForUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	protected, err := s.DB.Permissions.UserIsProtected(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":            u.ID,
		"email":         u.Email,
		"lang":          u.PreferredLanguage,
		"display_name":  u.DisplayName,
		"banned_until":  u.BannedUntil,
		"avatar_hash":   u.AvatarHash,
		"permissions":   actor.PermissionCodes(),
		"protected":     protected,
		"profile_count": profileCount,
		"texture_count": textureCount,
	}, nil
}

func (s AccountService) UpdateSelf(ctx context.Context, actor permission.Actor, body map[string]any) error {
	if err := actor.Require(accountUpdateSelfPermission); err != nil {
		return permissionDenied()
	}
	fields, err := s.normalizedSelfUpdateFields(ctx, actor.UserID, body)
	if err != nil {
		return err
	}
	if err := s.DB.Users.Update(ctx, actor.UserID, fields); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
		}
		if userstore.IsEmailConflict(err) {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "register", Reason: "already_exists"}
		}
		if errors.Is(err, userstore.ErrDisplayNameConflict) {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "username", Operation: "reserve", Reason: "conflict"}
		}
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) SendEmailChangeCode(ctx context.Context, actor permission.Actor, email string) (map[string]any, error) {
	if err := actor.Require(accountUpdateSelfPermission); err != nil {
		return nil, permissionDenied()
	}
	email, err := s.validateEmailChangeTarget(ctx, actor.UserID, email)
	if err != nil {
		return nil, err
	}
	return s.Verification.SendEmailChange(ctx, email)
}

func (s AccountService) ChangeEmailSelf(ctx context.Context, actor permission.Actor, email, code string) error {
	if err := actor.Require(accountUpdateSelfPermission); err != nil {
		return permissionDenied()
	}
	email, err := s.validateEmailChangeTarget(ctx, actor.UserID, email)
	if err != nil {
		return err
	}
	consumed, err := s.Verification.Consume(ctx, email, code, verificationsvc.PurposeEmailChange)
	if err != nil {
		return err
	}
	if !consumed {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "verification_code", Operation: "verify", Reason: "invalid"}
	}
	restoreCode := func() {
		_ = s.Verification.Restore(ctx, email, code, verificationsvc.PurposeEmailChange)
	}
	if err := s.DB.Users.Update(ctx, actor.UserID, map[string]any{"email": email}); err != nil {
		restoreCode()
		if errors.Is(err, pgx.ErrNoRows) {
			return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
		}
		if userstore.IsEmailConflict(err) {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "register", Reason: "already_exists"}
		}
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) ChangePasswordSelf(ctx context.Context, actor permission.Actor, oldPassword, newPassword string) error {
	if err := actor.Require(passwordUpdateSelfPermission); err != nil {
		return permissionDenied()
	}
	u, err := s.DB.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if u == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	if !util.VerifyPassword(oldPassword, u.Password) {
		return util.HTTPError{Status: http.StatusForbidden, Object: "password", Operation: "verify", Reason: "invalid"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, actor.UserID); err != nil {
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, actor.UserID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) DeleteSelf(ctx context.Context, actor permission.Actor) error {
	if err := actor.Require(accountDeleteSelfPermission); err != nil {
		return permissionDenied()
	}
	u, err := s.DB.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if u == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if isProtected {
		return util.HTTPError{Status: http.StatusForbidden, Object: "protected_subject", Operation: "delete", Reason: "denied"}
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, actor.UserID); err != nil {
		return err
	}
	externalIdentityIDs, err := s.deleteExternalIdentityAccessTokens(ctx, actor.UserID)
	if err != nil {
		return err
	}
	oauthClientIDs, err := s.prepareUserOAuthAccessTokens(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if err := s.Redis.InvalidateAuthUser(ctx, actor.UserID); err != nil {
		return err
	}
	ok, err := s.DB.Users.Delete(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	reportPostCommitError("remove self-deleted user sessions", s.Redis.DeleteYggTokensByUser(ctx, actor.UserID))
	for _, identityID := range externalIdentityIDs {
		reportPostCommitError("remove self-deleted external identity token races", s.Redis.DeleteExternalAccessToken(ctx, identityID))
	}
	for _, clientID := range oauthClientIDs {
		reportPostCommitError("remove self-deleted owner client access-token races", s.Redis.DeleteOAuthAccessTokensByClient(ctx, clientID))
	}
	reportPostCommitError("remove self-deleted user access-token races", s.Redis.DeleteOAuthAccessTokensByUser(ctx, actor.UserID))
	reportPostCommitError("invalidate self-deleted user cache races", s.Redis.InvalidateAuthUser(ctx, actor.UserID))
	return nil
}

func (s AccountService) normalizedSelfUpdateFields(ctx context.Context, userID string, body map[string]any) (map[string]any, error) {
	fields := map[string]any{}
	if _, ok := body["email"]; ok {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "update", Reason: "denied"}
	}
	if v, ok := body["display_name"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "username", Operation: "validate", Reason: "required"}
		}
		taken, err := s.DB.Users.IsDisplayNameTaken(ctx, v, userID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "username", Operation: "reserve", Reason: "conflict"}
		}
		fields["display_name"] = v
	}
	if v, ok := body["preferred_language"].(string); ok && v != "" {
		fields["preferred_language"] = v
	}
	if v, ok := body["avatar_hash"]; ok {
		if v == nil {
			fields["avatar_hash"] = nil
		} else if hash, ok := v.(string); ok {
			if hash == "" {
				fields["avatar_hash"] = nil
			} else {
				exists, err := s.DB.Textures.VerifyOwnership(ctx, userID, hash, "skin")
				if err != nil {
					return nil, err
				}
				if !exists {
					return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "avatar", Operation: "resolve", Reason: "not_found"}
				}
				fields["avatar_hash"] = hash
			}
		} else {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "avatar", Operation: "configure", Reason: "invalid"}
		}
	}
	return fields, nil
}

func (s AccountService) validateEmailChangeTarget(ctx context.Context, userID, email string) (string, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	email = strings.TrimSpace(email)
	if !util.ValidEmail(email) {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "validate", Reason: "invalid"}
	}
	policy := s.EmailPolicy
	if policy.DB == nil {
		policy.DB = s.DB
	}
	if err := policy.RequireAllowed(ctx, email); err != nil {
		return "", err
	}
	if strings.EqualFold(user.Email, email) {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "update", Reason: "conflict"}
	}
	existing, err := s.DB.Users.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "register", Reason: "already_exists"}
	}
	return email, nil
}
