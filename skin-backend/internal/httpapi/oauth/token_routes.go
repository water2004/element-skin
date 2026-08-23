package oauth

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (h Handler) Token(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	res, err := h.oauth.IssueToken(req.Context(), oauthsvc.TokenRequest{
		GrantType:    req.Form.Get("grant_type"),
		Code:         req.Form.Get("code"),
		RedirectURI:  req.Form.Get("redirect_uri"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		CodeVerifier: req.Form.Get("code_verifier"),
		RefreshToken: req.Form.Get("refresh_token"),
		Scope:        req.Form.Get("scope"),
		DeviceCode:   req.Form.Get("device_code"),
	})
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) Revoke(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	if err := h.oauth.RevokeToken(req.Context(), clientID, clientSecret, req.Form.Get("token")); err != nil {
		writeProtocolError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h Handler) Introspect(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	res, err := h.oauth.Introspect(req.Context(), shared.CurrentActor(req), req.Form.Get("token"))
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func clientCredentials(req *http.Request) (string, string) {
	if id, secret, ok := req.BasicAuth(); ok {
		return id, secret
	}
	return strings.TrimSpace(req.Form.Get("client_id")), req.Form.Get("client_secret")
}
