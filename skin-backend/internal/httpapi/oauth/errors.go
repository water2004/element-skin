package oauth

import (
	"errors"
	"net/http"

	"element-skin/backend/internal/util"
)

type protocolErrorBody struct {
	Error string `json:"error"`
}

func writeProtocolError(w http.ResponseWriter, err error) {
	code, status := protocolError(err)
	if code == "invalid_client" {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
	}
	util.JSON(w, status, protocolErrorBody{Error: code})
}

func protocolError(err error) (string, int) {
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) {
		return "server_error", http.StatusInternalServerError
	}
	code := protocolErrorCode(httpErr)
	return code, protocolErrorStatus(code, httpErr.Status)
}

func protocolErrorCode(err util.HTTPError) string {
	switch {
	case errorIs(err, "oauth_authorization", "grant", "incomplete"):
		return "authorization_pending"
	case errorIs(err, "oauth_authorization", "grant", "denied"), errorIs(err, "permission", "check", "denied"):
		return "access_denied"
	case errorIs(err, "device_code", "verify", "expired"):
		return "expired_token"
	case errorIs(err, "grant_type", "validate", "unsupported"):
		return "unsupported_grant_type"
	case errorIs(err, "client_id", "verify", "invalid"), errorIs(err, "client_secret", "verify", "invalid"):
		return "invalid_client"
	case errorIs(err, "authorization_code", "verify", "invalid"),
		errorIs(err, "refresh_token", "verify", "invalid"),
		errorIs(err, "device_code", "verify", "invalid"),
		errorIs(err, "code_verifier", "verify", "invalid"),
		errorIs(err, "oauth_grant", "exchange", "invalid"):
		return "invalid_grant"
	case errorIs(err, "oauth_scope", "validate", "required"),
		errorIs(err, "oauth_scope", "validate", "invalid"),
		errorIs(err, "oauth_scope", "authorize", "denied"):
		return "invalid_scope"
	case errorIs(err, "oauth_client", "authenticate", "unsupported"):
		return "unauthorized_client"
	default:
		if err.Status == http.StatusForbidden {
			return "access_denied"
		}
		if err.Status >= http.StatusInternalServerError {
			return "server_error"
		}
		return "invalid_request"
	}
}

func errorIs(err util.HTTPError, object, operation, reason string) bool {
	return err.Object == object && err.Operation == operation && err.Reason == reason
}

func protocolErrorStatus(code string, fallback int) int {
	switch code {
	case "invalid_client":
		return http.StatusUnauthorized
	case "server_error":
		return http.StatusInternalServerError
	default:
		if fallback >= http.StatusInternalServerError {
			return http.StatusInternalServerError
		}
		return http.StatusBadRequest
	}
}
