package account

import (
	"context"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s AccountService) DeleteUser(ctx context.Context, actor permission.Actor, targetID string) error {
	if err := actor.Require(accountDeletePermission); err != nil {
		return permissionDenied()
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot delete yourself"}
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, target.ID); err != nil {
		return err
	}
	externalIdentityIDs, err := s.deleteExternalIdentityAccessTokens(ctx, target.ID)
	if err != nil {
		return err
	}
	oauthClientIDs, err := s.prepareUserOAuthAccessTokens(ctx, target.ID)
	if err != nil {
		return err
	}
	if err := s.Redis.InvalidateAuthUser(ctx, target.ID); err != nil {
		return err
	}
	ok, err := s.DB.Users.Delete(ctx, target.ID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	reportPostCommitError("remove deleted user sessions", s.Redis.DeleteYggTokensByUser(ctx, target.ID))
	for _, identityID := range externalIdentityIDs {
		reportPostCommitError("remove deleted external identity token races", s.Redis.DeleteExternalAccessToken(ctx, identityID))
	}
	for _, clientID := range oauthClientIDs {
		reportPostCommitError("remove deleted owner client access-token races", s.Redis.DeleteOAuthAccessTokensByClient(ctx, clientID))
	}
	reportPostCommitError("remove deleted user access-token races", s.Redis.DeleteOAuthAccessTokensByUser(ctx, target.ID))
	reportPostCommitError("invalidate deleted user cache races", s.Redis.InvalidateAuthUser(ctx, target.ID))
	return nil
}

func (s AccountService) ResetPassword(ctx context.Context, actor permission.Actor, input ResetPasswordInput) error {
	if err := actor.Require(accountUpdatePermission); err != nil {
		return permissionDenied()
	}
	userID := strings.TrimSpace(input.UserID)
	newPassword := input.NewPassword
	if userID == "" || newPassword == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "user_id and new_password required"}
	}
	target, err := s.modifiableUser(ctx, actor, userID)
	if err != nil {
		return err
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, target.ID); err != nil {
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, target.ID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	return s.Redis.InvalidateAuthUser(ctx, target.ID)
}

func (s AccountService) BanUser(ctx context.Context, actor permission.Actor, targetID string, input BanUserInput) (int64, error) {
	if err := actor.Require(accountBanPermission); err != nil {
		return 0, permissionDenied()
	}
	if input.BannedUntil < time.Now().Add(-24*time.Hour).UnixMilli() {
		return 0, util.HTTPError{Status: http.StatusBadRequest, Detail: "banned_until is required"}
	}
	reason, err := normalizedBanReason(input.Reason)
	if err != nil {
		return 0, err
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return 0, err
	}
	if err := s.DB.Users.Ban(ctx, target.ID, input.BannedUntil); err != nil {
		if database.IsNoRows(err) {
			return 0, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
		}
		return 0, err
	}
	if err := s.Redis.InvalidateAuthUser(ctx, target.ID); err != nil {
		return 0, err
	}
	if err := s.createBanNotice(ctx, target.ID, input.BannedUntil, reason); err != nil {
		return 0, err
	}
	return input.BannedUntil, nil
}

func (s AccountService) UnbanUser(ctx context.Context, actor permission.Actor, targetID string) error {
	if err := actor.Require(accountUnbanPermission); err != nil {
		return permissionDenied()
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return err
	}
	if err := s.DB.Users.Unban(ctx, target.ID); err != nil {
		if database.IsNoRows(err) {
			return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
		}
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, target.ID)
}

func (s AccountService) prepareUserOAuthAccessTokens(ctx context.Context, userID string) ([]string, error) {
	clientIDs, err := s.DB.OAuth.ClientIDsByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.DeleteOAuthAccessTokensByUser(ctx, userID); err != nil {
		return nil, err
	}
	for _, clientID := range clientIDs {
		if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, clientID); err != nil {
			return nil, err
		}
	}
	return clientIDs, nil
}

func (s AccountService) deleteExternalIdentityAccessTokens(ctx context.Context, userID string) ([]string, error) {
	items, err := s.DB.Identities.ListIdentitiesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	identityIDs := make([]string, 0, len(items))
	for _, item := range items {
		identityIDs = append(identityIDs, item.ID)
		if err := s.Redis.DeleteExternalAccessToken(ctx, item.ID); err != nil {
			return nil, err
		}
	}
	return identityIDs, nil
}

func normalizedBanReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason is required"}
	}
	if len([]rune(reason)) > accountBanReasonMaxRunes {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason too long"}
	}
	return reason, nil
}
