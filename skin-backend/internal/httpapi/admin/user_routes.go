package admin

import (
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Users(w http.ResponseWriter, req *http.Request) {
	cursor, err := util.DecodeCursor(req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	res, err := h.db.ListUsers(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) ToggleUserAdmin(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot change your own admin status"})
		return
	}
	next, err := h.db.ToggleAdmin(req.Context(), targetID)
	if err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "is_admin": next})
}

func (h Handler) DeleteUser(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot delete yourself"})
		return
	}
	ok, err := h.db.DeleteUser(req.Context(), targetID)
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

func (h Handler) UserProfiles(w http.ResponseWriter, req *http.Request) {
	res, err := h.db.ListProfilesByUser(req.Context(), req.PathValue("user_id"), util.ClampLimit(req.URL.Query().Get("limit")), req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) BanUser(w http.ResponseWriter, req *http.Request) {
	var body map[string]int64
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	until, ok := body["banned_until"]
	if !ok || until < time.Now().Add(-24*time.Hour).UnixMilli() {
		util.Error(w, util.HTTPError{Status: 400, Detail: "banned_until is required"})
		return
	}
	if err := h.db.BanUser(req.Context(), req.PathValue("user_id"), until); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "banned_until": until})
}

func (h Handler) UnbanUser(w http.ResponseWriter, req *http.Request) {
	user, err := h.db.GetUserByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.db.UnbanUser(req.Context(), user.ID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ResetUserPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	userID := body["user_id"]
	newPassword := body["new_password"]
	if userID == "" || newPassword == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "user_id and new_password required"})
		return
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		util.Error(w, err)
		return
	}
	ok, err := h.db.UpdatePasswordAndRevokeRefresh(req.Context(), userID, hash)
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
