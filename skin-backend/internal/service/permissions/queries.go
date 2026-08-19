package permissions

import (
	"context"
	"net/http"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s PermissionService) UserPermissions(ctx context.Context, actor permission.Actor, targetID string) (UserPermissionsResponse, error) {
	if err := actor.Require(permissionReadAny); err != nil {
		return UserPermissionsResponse{}, permissionDenied()
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return UserPermissionsResponse{}, err
	} else if !ok {
		return UserPermissionsResponse{}, util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	roles, err := s.DB.Permissions.RoleIDsForUser(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	bits, err := s.DB.Permissions.EffectivePermissionsForUser(ctx, targetID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForUser(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	protected, err := s.DB.Permissions.UserIsProtected(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	return UserPermissionsResponse{
		Roles:                roles,
		Protected:            protected,
		EffectivePermissions: permissionCodesFromBitSet(bits),
		Overrides:            permissionOverrideResponses(overrides),
		Catalog:              permissionCatalog(),
	}, nil
}
