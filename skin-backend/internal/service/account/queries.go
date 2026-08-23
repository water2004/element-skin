package account

import (
	"context"
	"net/http"
	"strings"

	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s AccountService) ListUsers(ctx context.Context, actor permission.Actor, cursor string, limit int, query string) (map[string]any, error) {
	if err := actor.Require(userReadAnyPermission); err != nil {
		return nil, permissionDenied()
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Users.List(ctx, limit, last, strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	if err := s.attachRoles(ctx, res["items"]); err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s AccountService) UserDetail(ctx context.Context, actor permission.Actor, userID string) (map[string]any, error) {
	if err := actor.Require(accountReadAnyPermission); err != nil {
		return nil, permissionDenied()
	}
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	out := userstore.PublicUser(*user)
	roles, err := s.DB.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	protected, err := s.DB.Permissions.UserIsProtected(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out["roles"] = roles
	out["protected"] = protected
	return out, nil
}

func (s AccountService) attachRoles(ctx context.Context, rawItems any) error {
	items, _ := rawItems.([]map[string]any)
	for _, item := range items {
		userID, _ := item["id"].(string)
		if userID == "" {
			continue
		}
		roles, err := s.DB.Permissions.RoleIDsForUser(ctx, userID)
		if err != nil {
			return err
		}
		protected, err := s.DB.Permissions.UserIsProtected(ctx, userID)
		if err != nil {
			return err
		}
		item["roles"] = roles
		item["protected"] = protected
	}
	return nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
