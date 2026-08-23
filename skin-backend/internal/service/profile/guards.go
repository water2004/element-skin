package profile

import (
	"net/http"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func requireActorPermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func requireOwnedOrBoundProfilePermission(actor permission.Actor, profileID string, owned, bound permission.Definition) error {
	if actor.Has(owned) {
		return nil
	}
	if actor.BoundProfileID == profileID && actor.Has(bound) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func asCursorMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
