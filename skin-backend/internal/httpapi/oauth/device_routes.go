package oauth

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (h Handler) DeviceCode(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Detail: "invalid form"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	res, err := h.oauth.StartDeviceAuthorization(req.Context(), oauthsvc.DeviceAuthorizationRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        req.Form.Get("scope"),
	})
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	base := strings.TrimRight(h.cfg.SiteURL, "/")
	if base == "" {
		base = h.baseURL()
	}
	out := map[string]any{
		"device_code":               res.DeviceCode,
		"user_code":                 res.UserCode,
		"verification_uri":          base + "/oauth/device",
		"verification_uri_complete": base + "/oauth/device?user_code=" + res.UserCode,
		"expires_in":                res.ExpiresIn,
		"interval":                  res.Interval,
		"scope":                     res.Scope,
		"permissions":               res.Permissions,
	}
	util.JSON(w, http.StatusOK, out)
}

func (h Handler) DeviceInfo(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.DeviceAuthorizationDetails(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("user_code"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) DeviceDecision(w http.ResponseWriter, req *http.Request) {
	var body deviceDecisionBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.oauth.DecideDeviceAuthorization(req.Context(), shared.CurrentActor(req), oauthsvc.DeviceDecisionRequest{
		UserCode: body.UserCode,
		Approve:  body.Approve,
	}); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

type deviceDecisionBody struct {
	UserCode string `json:"user_code"`
	Approve  bool   `json:"approve"`
}
