package oauth

import (
	"errors"
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) OpenIDConfiguration(w http.ResponseWriter, _ *http.Request) {
	base := h.baseURL()
	util.JSON(w, http.StatusOK, map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"userinfo_endpoint":                     base + "/oauth/userinfo",
		"jwks_uri":                              base + "/oauth/jwks",
		"revocation_endpoint":                   base + "/oauth/revoke",
		"introspection_endpoint":                base + "/oauth/introspect",
		"response_types_supported":              []string{"code"},
		"response_modes_supported":              []string{"query"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"subject_types_supported":               []string{"pairwise"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      h.authorizationScopeCodes(),
		"claims_supported": []string{
			"sub", "iss", "aud", "exp", "iat", "nonce", "name", "preferred_username",
			"locale", "email", "email_verified",
		},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (h Handler) JWKS(w http.ResponseWriter, _ *http.Request) {
	if h.oauth.OIDCSigner == nil {
		util.Error(w, errors.New("OIDC signing key is not loaded"))
		return
	}
	util.JSON(w, http.StatusOK, h.oauth.OIDCSigner.JWKS())
}

func (h Handler) UserInfo(w http.ResponseWriter, req *http.Request) {
	bearer, ok := shared.BearerToken(req)
	if !ok || strings.TrimSpace(bearer) == "" {
		writeUserInfoError(w)
		return
	}
	claims, err := h.oauth.UserInfo(req.Context(), bearer)
	if err != nil {
		var httpErr util.HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusUnauthorized {
			writeUserInfoError(w)
			return
		}
		writeProtocolError(w, err)
		return
	}
	util.JSON(w, http.StatusOK, claims)
}

func writeUserInfoError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	util.JSON(w, http.StatusUnauthorized, protocolErrorBody{Error: "invalid_token"})
}
