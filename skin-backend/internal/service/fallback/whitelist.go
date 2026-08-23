package fallback

import (
	"context"
	"net/http"
	"strings"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	officialWhitelistReadPermission   = permission.MustDefinitionByCode("official_whitelist.read.any")
	officialWhitelistAddPermission    = permission.MustDefinitionByCode("official_whitelist.add.any")
	officialWhitelistRemovePermission = permission.MustDefinitionByCode("official_whitelist.remove.any")
)

type WhitelistInput struct {
	Username   string
	EndpointID int
}

func (f Fallback) ListWhitelistUsers(ctx context.Context, actor permission.Actor, endpointID int) ([]map[string]any, error) {
	if err := requirePermission(actor, officialWhitelistReadPermission); err != nil {
		return nil, err
	}
	if endpointID <= 0 {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "fallback_endpoint", Operation: "configure", Reason: "required"}
	}
	users, err := f.DB.Fallbacks.ListWhitelistUsers(ctx, endpointID)
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = []map[string]any{}
	}
	return users, nil
}

func (f Fallback) AddWhitelistUser(ctx context.Context, actor permission.Actor, input WhitelistInput) error {
	if err := requirePermission(actor, officialWhitelistAddPermission); err != nil {
		return err
	}
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "username", Operation: "validate", Reason: "required"}
	}
	if input.EndpointID <= 0 {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "fallback_endpoint", Operation: "configure", Reason: "required"}
	}
	if err := f.DB.Fallbacks.AddWhitelistUser(ctx, username, input.EndpointID); err != nil {
		if dbfallback.IsEndpointNotFound(err) {
			return util.HTTPError{Status: http.StatusNotFound, Object: "fallback_endpoint", Operation: "resolve", Reason: "not_found"}
		}
		return err
	}
	return nil
}

func (f Fallback) RemoveWhitelistUser(ctx context.Context, actor permission.Actor, username string, endpointID int) error {
	if err := requirePermission(actor, officialWhitelistRemovePermission); err != nil {
		return err
	}
	if endpointID <= 0 {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "fallback_endpoint", Operation: "configure", Reason: "required"}
	}
	return f.DB.Fallbacks.RemoveWhitelistUser(ctx, username, endpointID)
}

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
