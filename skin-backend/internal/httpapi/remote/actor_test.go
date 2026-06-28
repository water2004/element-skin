package remote_test

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
)

func withUserActor(req *http.Request, userID string) *http.Request {
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, rolePermissions(permission.RoleUser)...))
}

func rolePermissions(roleID string) []permission.Definition {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return role.Permissions
		}
	}
	panic("missing role: " + roleID)
}
