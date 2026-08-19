package permissions

import (
	"context"
	"net/http"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s PermissionService) userExists(ctx context.Context, userID string) (bool, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func ensurePermissionOverrideAllowed(actor permission.Actor, targetID string, def permission.Definition) error {
	if def.Code == manageProtectedPermission.Code {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "protected_permission", Operation: "transfer", Reason: "required"}
	}
	if !protectedPermission(def) {
		return nil
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Object: "protected_permission", Operation: "update", Reason: "denied"}
	}
	if !actor.Has(manageProtectedPermission) {
		return util.HTTPError{Status: http.StatusForbidden, Object: "protected_permission", Operation: "manage", Reason: "required"}
	}
	return nil
}

func protectedPermission(def permission.Definition) bool {
	return def.Scope.ID == permission.ScopeSystem || def.Resource.ID == permission.ResourcePermissionProtected
}

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
