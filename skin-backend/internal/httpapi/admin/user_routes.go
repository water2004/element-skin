package admin

import (
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Users(w http.ResponseWriter, req *http.Request) {
	rawCursor := req.URL.Query().Get("cursor")
	cursor, err := util.DecodeCursor(rawCursor)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	if rawCursor != "" && last == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Users.List(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) User(w http.ResponseWriter, req *http.Request) {
	user, err := h.db.Users.GetByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, userstore.PublicUser(*user))
}

func (h Handler) ToggleUserAdmin(w http.ResponseWriter, req *http.Request) {
	if !shared.CurrentUserIsSuperAdmin(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "super admin required"})
		return
	}
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot change your own admin status"})
		return
	}
	next, err := h.db.Users.ToggleAdmin(req.Context(), targetID)
	if err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "is_admin": next})
}

func (h Handler) TransferSuperAdmin(w http.ResponseWriter, req *http.Request) {
	if !shared.CurrentUserIsSuperAdmin(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "super admin required"})
		return
	}
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 400, Detail: "target is already current super admin"})
		return
	}
	target, err := h.db.Users.GetByID(req.Context(), targetID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if target == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	currentID := shared.CurrentUserID(req)
	if err := h.invalidateAuthUsers(req, currentID, targetID); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.db.Users.TransferSuperAdmin(req.Context(), currentID, targetID); err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.invalidateAuthUsers(req, currentID, targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) invalidateAuthUsers(req *http.Request, userIDs ...string) error {
	var firstErr error
	for _, userID := range userIDs {
		if err := h.redis.InvalidateAuthUser(req.Context(), userID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h Handler) DeleteUser(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot delete yourself"})
		return
	}
	if err := h.ensureTargetNotSuperAdmin(req, targetID); err != nil {
		util.Error(w, err)
		return
	}
	ok, err := h.site.DeleteUser(req.Context(), targetID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) UserProfiles(w http.ResponseWriter, req *http.Request) {
	rawCursor := req.URL.Query().Get("cursor")
	cursor, err := util.DecodeCursor(rawCursor)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	lastID := ""
	if cursor != nil {
		lastID, _ = cursor["last_id"].(string)
	}
	if rawCursor != "" && lastID == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Profiles.ListByUser(req.Context(), req.PathValue("user_id"), util.ClampLimit(req.URL.Query().Get("limit")), lastID)
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
	userID := req.PathValue("user_id")
	if err := h.ensureTargetNotSuperAdmin(req, userID); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.db.Users.Ban(req.Context(), userID, until); err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "banned_until": until})
}

func (h Handler) UnbanUser(w http.ResponseWriter, req *http.Request) {
	user, err := h.db.Users.GetByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if user.IsSuperAdmin && !shared.CurrentUserIsSuperAdmin(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot modify super admin"})
		return
	}
	if err := h.db.Users.Unban(req.Context(), user.ID); err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), user.ID); err != nil {
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
	if err := h.ensureTargetNotSuperAdmin(req, userID); err != nil {
		util.Error(w, err)
		return
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.DeleteYggTokensByUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	ok, err := h.db.Users.UpdatePasswordAndRevokeRefresh(req.Context(), userID, hash)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ensureTargetNotSuperAdmin(req *http.Request, targetID string) error {
	target, err := h.db.Users.GetByID(req.Context(), targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return util.HTTPError{Status: 404, Detail: "user not found"}
	}
	if target.IsSuperAdmin && !shared.CurrentUserIsSuperAdmin(req) {
		return util.HTTPError{Status: 403, Detail: "cannot modify super admin"}
	}
	return nil
}
