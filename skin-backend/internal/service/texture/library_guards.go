package texture

import (
	"errors"
	"net/http"

	"element-skin/backend/internal/database"
	texturedb "element-skin/backend/internal/database/texture"
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

func requireBoundProfilePermission(actor permission.Actor, profileID string, def permission.Definition) error {
	if actor.BoundProfileID == profileID && actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func textureNotFoundError(err error) error {
	if errors.Is(err, texturedb.ErrNotFound) {
		return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	return err
}

func sameHash(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func profileUpdateError(err error) error {
	if database.IsNoRows(err) {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	return err
}
