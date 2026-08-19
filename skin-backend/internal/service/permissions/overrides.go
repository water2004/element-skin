package permissions

import (
	"context"
	"net/http"

	"element-skin/backend/internal/permission"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (s PermissionService) SetUserPermissionOverride(ctx context.Context, actor permission.Actor, targetID, code, effect string) error {
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "permission", Operation: "resolve", Reason: "not_found"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	switch effect {
	case "allow":
		if err := actor.Require(permissionGrantAny); err != nil {
			return permissionDenied()
		}
	case "deny":
		if err := actor.Require(permissionRevokeAny); err != nil {
			return permissionDenied()
		}
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Object: "permission_override", Operation: "configure", Reason: "invalid"}
	}
	if err := ensurePermissionOverrideAllowed(actor, targetID, def); err != nil {
		return err
	}
	currentEffect, err := s.permissionOverrideEffect(ctx, targetID, code)
	if err != nil {
		return err
	}
	if currentEffect == effect {
		return nil
	}
	if err := s.DB.Permissions.SetSubjectPermissionOverride(ctx, targetID, def, effect, actor.SubjectID); err != nil {
		return err
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createOverrideChangeNotice(ctx, targetID, def, effect); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s PermissionService) ClearUserPermissionOverride(ctx context.Context, actor permission.Actor, targetID, code string) error {
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "permission", Operation: "resolve", Reason: "not_found"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	effect, err := s.permissionOverrideEffect(ctx, targetID, code)
	if err != nil {
		return err
	}
	if effect == "" {
		return util.HTTPError{Status: http.StatusNotFound, Object: "permission_override", Operation: "resolve", Reason: "not_found"}
	}
	if effect == "allow" {
		if err := actor.Require(permissionRevokeAny); err != nil {
			return permissionDenied()
		}
	} else if err := actor.Require(permissionGrantAny); err != nil {
		return permissionDenied()
	}
	if err := ensurePermissionOverrideAllowed(actor, targetID, def); err != nil {
		return err
	}
	cleared, err := s.DB.Permissions.ClearSubjectPermissionOverride(ctx, targetID, def)
	if err != nil {
		return err
	}
	if !cleared {
		return util.HTTPError{Status: http.StatusNotFound, Object: "permission_override", Operation: "resolve", Reason: "not_found"}
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createOverrideClearNotice(ctx, targetID, def, effect); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s PermissionService) reconcileOAuthAfterUserPermissionChange(ctx context.Context, userID string) error {
	_, err := (oauthsvc.Service{DB: s.DB, Redis: s.Redis}).ReconcileUserPermissionDependents(ctx, userID)
	return err
}

func (s PermissionService) permissionOverrideEffect(ctx context.Context, userID, code string) (string, error) {
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	for _, override := range overrides {
		if override.PermissionCode == code {
			return override.Effect, nil
		}
	}
	return "", nil
}
