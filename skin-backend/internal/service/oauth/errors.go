package oauth

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func oauthError(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func badRequest(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func forbidden() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func unauthorized(detail string) error {
	return util.HTTPError{Status: http.StatusUnauthorized, Detail: detail}
}

func notFound(detail string) error {
	return util.HTTPError{Status: http.StatusNotFound, Detail: detail}
}
