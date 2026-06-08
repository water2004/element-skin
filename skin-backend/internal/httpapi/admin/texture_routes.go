package admin

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Textures(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastHash, err := shared.CursorCreatedHash(req.URL.Query().Get("cursor"), "last_skin_hash")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Textures.ListAll(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), lastCreated, lastHash, strings.TrimSpace(req.URL.Query().Get("q")), req.URL.Query().Get("type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) UpdateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	hash := req.PathValue("hash")
	updated := false
	if v, ok := body["note"].(string); ok {
		if err := h.db.Textures.AdminUpdateNote(req.Context(), hash, v); err != nil {
			if err == texture.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["model"].(string); ok {
		if v != "default" && v != "slim" {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid model"})
			return
		}
		if err := h.db.Textures.AdminUpdateModel(req.Context(), hash, v); err != nil {
			if err == texture.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["is_public"]; ok {
		if !shared.ValidPublicValue(v) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid is_public"})
			return
		}
		pub := shared.PublicBool(v)
		if err := h.db.Textures.AdminUpdatePublic(req.Context(), hash, pub); err != nil {
			if err == texture.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if !updated {
		util.Error(w, util.HTTPError{Status: 400, Detail: "至少需要一个更新字段: model, note, is_public"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	force := req.URL.Query().Get("force") == "true"
	typ := req.URL.Query().Get("type")
	if typ == "" {
		typ = "skin"
	}
	if err := h.db.Textures.AdminDelete(req.Context(), req.PathValue("hash"), typ, req.URL.Query().Get("user_id"), force); err != nil {
		if strings.Contains(err.Error(), "user_id") {
			util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
			return
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"success": true})
}
