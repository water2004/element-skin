package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Me(w http.ResponseWriter, req *http.Request) {
	res, err := h.site.Me(req.Context(), shared.CurrentUserID(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateMe(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.site.UpdateMe(req.Context(), shared.CurrentUserID(req), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteMe(w http.ResponseWriter, req *http.Request) {
	userID := shared.CurrentUserID(req)
	user, err := h.db.GetUserByID(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if user.IsAdmin {
		util.Error(w, util.HTTPError{Status: 403, Detail: "管理员不能删除自己的账号"})
		return
	}
	ok, err := h.db.DeleteUser(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ChangePassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.site.ChangePassword(req.Context(), shared.CurrentUserID(req), body["old_password"], body["new_password"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "message": "密码修改成功"})
}
