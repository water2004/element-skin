package account

import (
	"context"
	"net/http"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s AccountService) modifiableUser(ctx context.Context, actor permission.Actor, targetID string) (*model.User, error) {
	target, err := s.DB.Users.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return nil, util.HTTPError{Status: http.StatusForbidden, Object: "protected_subject", Operation: "update", Reason: "denied"}
	}
	return target, nil
}

func (s AccountService) ensureProtectedSubjectMutationAllowed(ctx context.Context, actor permission.Actor, targetID string) error {
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, targetID)
	if err != nil {
		return err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return util.HTTPError{Status: http.StatusForbidden, Object: "protected_subject", Operation: "update", Reason: "denied"}
	}
	return nil
}

func (s AccountService) userExists(ctx context.Context, userID string) (bool, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func ensureRoleMutationAllowed(actor permission.Actor, roleID string) error {
	if roleID == permission.RoleSystemMaintenance {
		if !actor.Has(manageProtectedPermission) {
			return util.HTTPError{Status: http.StatusForbidden, Object: "protected_role", Operation: "manage", Reason: "required"}
		}
	}
	return nil
}

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
