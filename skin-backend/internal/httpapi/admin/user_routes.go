package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/util"
)

func (h Handler) Users(w http.ResponseWriter, req *http.Request) {
	res, err := h.accounts.ListUsers(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit"), 15), req.URL.Query().Get("q"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) User(w http.ResponseWriter, req *http.Request) {
	out, err := h.accounts.UserDetail(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, out)
}

func (h Handler) GrantUserRole(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if err := h.accounts.GrantUserRole(req.Context(), shared.CurrentActor(req), targetID, roleID); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) RevokeUserRole(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if err := h.accounts.RevokeUserRole(req.Context(), shared.CurrentActor(req), targetID, roleID); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) TransferProtectedSubject(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if err := h.accounts.TransferProtectedSubject(req.Context(), shared.CurrentActor(req), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) UserPermissions(w http.ResponseWriter, req *http.Request) {
	res, err := h.perms.UserPermissions(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SetUserPermissionOverride(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Effect string `json:"effect"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	code := req.PathValue("permission_code")
	if err := h.perms.SetUserPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), code, body.Effect); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ClearUserPermissionOverride(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("permission_code")
	if err := h.perms.ClearUserPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), code); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) DeleteUser(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if err := h.accounts.DeleteUser(req.Context(), shared.CurrentActor(req), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) UserProfiles(w http.ResponseWriter, req *http.Request) {
	res, err := h.profiles.ListProfilesByUser(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit")))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) BanUser(w http.ResponseWriter, req *http.Request) {
	var body struct {
		BannedUntil int64  `json:"banned_until"`
		Reason      string `json:"reason"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	until, err := h.accounts.BanUser(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), accountsvc.BanUserInput{
		BannedUntil: body.BannedUntil,
		Reason:      body.Reason,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"banned_until": until})
}

func (h Handler) UnbanUser(w http.ResponseWriter, req *http.Request) {
	if err := h.accounts.UnbanUser(req.Context(), shared.CurrentActor(req), req.PathValue("user_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ResetUserPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if err := h.accounts.ResetPassword(req.Context(), shared.CurrentActor(req), accountsvc.ResetPasswordInput{
		UserID:      body["user_id"],
		NewPassword: body["new_password"],
	}); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
