package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Login(w http.ResponseWriter, req *http.Request) {
	if !h.checkAuthRateLimit(w, req, "login") {
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	res, err := h.authSvc.Login(req.Context(), body["email"], body["password"])
	if err != nil {
		util.Error(w, err)
		return
	}
	shared.SetWebSessionCookies(w, h.cfg, res["access_token"].(string), res["refresh_token"].(string), res["refresh_max_age_seconds"].(int))
	util.JSON(w, 200, map[string]any{"user_id": res["user_id"], "permissions": res["permissions"]})
}

func (h Handler) Logout(w http.ResponseWriter, req *http.Request) {
	if c, err := req.Cookie("refresh_token"); err == nil {
		if err := h.authSvc.RevokeRefresh(req.Context(), c.Value); err != nil {
			util.Error(w, err)
			return
		}
	}
	shared.ClearWebSessionCookies(w, h.cfg)
	util.NoContent(w)
}

func (h Handler) Register(w http.ResponseWriter, req *http.Request) {
	if !h.checkAuthRateLimit(w, req, "register") {
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	id, err := h.authSvc.Register(
		req.Context(),
		body["email"],
		body["password"],
		body["username"],
		body["invite"],
		body["code"],
	)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h Handler) SendVerificationCode(w http.ResponseWriter, req *http.Request) {
	if !h.checkAuthRateLimit(w, req, "verification") {
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	email := body["email"]
	if email == "" {
		util.Error(w, util.HTTPError{Status: 400, Object: "email", Operation: "validate", Reason: "required"})
		return
	}
	res, err := h.authSvc.SendVerificationCode(req.Context(), email, body["type"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) ResetPassword(w http.ResponseWriter, req *http.Request) {
	if !h.checkAuthRateLimit(w, req, "reset") {
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if body["email"] == "" || body["password"] == "" || body["code"] == "" {
		util.Error(w, util.HTTPError{Status: 400, Object: "registration", Operation: "validate", Reason: "required"})
		return
	}
	if err := h.authSvc.ResetPassword(req.Context(), body["email"], body["password"], body["code"]); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) RefreshToken(w http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		util.Error(w, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"})
		return
	}
	res, err := h.authSvc.RotateRefresh(req.Context(), c.Value)
	if err != nil {
		util.Error(w, err)
		return
	}
	shared.SetWebSessionCookies(w, h.cfg, res["access_token"].(string), res["refresh_token"].(string), res["refresh_max_age_seconds"].(int))
	util.JSON(w, 200, map[string]any{"permissions": res["permissions"]})
}
