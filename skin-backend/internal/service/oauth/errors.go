package oauth

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func oauthError(object, operation, reason string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Object: object, Operation: operation, Reason: reason}
}

func badRequest(object, operation, reason string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Object: object, Operation: operation, Reason: reason}
}

func forbidden() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func unauthorized(object, operation, reason string) error {
	return util.HTTPError{Status: http.StatusUnauthorized, Object: object, Operation: operation, Reason: reason}
}

func notFound(object, operation, reason string) error {
	return util.HTTPError{Status: http.StatusNotFound, Object: object, Operation: operation, Reason: reason}
}
