package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) GetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := h.settings.GetGroup(req.Context(), "site")
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SaveSiteSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.settings.SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.settings.InvalidateCache(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicSettings(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) GetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := h.settings.GetGroup(req.Context(), req.PathValue("group"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SaveSettingsGroup(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.settings.SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.settings.InvalidateCache(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	switch req.PathValue("group") {
	case "site", "fallback", "email":
		if err := h.redis.InvalidatePublicSettings(req.Context()); err != nil {
			util.Error(w, err)
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
