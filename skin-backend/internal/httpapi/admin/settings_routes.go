package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

func (h Handler) GetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (settingssvc.Settings{DB: h.db}).GetGroup(req.Context(), "site")
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
	if err := (settingssvc.Settings{DB: h.db}).SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) GetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := (settingssvc.Settings{DB: h.db}).GetGroup(req.Context(), req.PathValue("group"))
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
	if err := (settingssvc.Settings{DB: h.db}).SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
