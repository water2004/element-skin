package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	profileReadOwnedPermission   = permission.MustDefinitionByCode("profile.read.owned")
	profileCreateOwnedPermission = permission.MustDefinitionByCode("profile.create.owned")
	profileUpdateOwnedPermission = permission.MustDefinitionByCode("profile.update.owned")
	profileDeleteOwnedPermission = permission.MustDefinitionByCode("profile.delete.owned")
	textureClearOwnedPermission  = permission.MustDefinitionByCode("texture.clear.owned")
)

func (h Handler) CreateProfile(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, profileCreateOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.site.CreateProfile(req.Context(), shared.CurrentActor(req), body["name"], body["model"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateProfile(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, profileUpdateOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.site.UpdateProfile(req.Context(), shared.CurrentActor(req), req.PathValue("pid"), body["name"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteProfile(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, profileDeleteOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.DeleteProfile(req.Context(), shared.CurrentActor(req), req.PathValue("pid")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ClearProfileSkin(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, textureClearOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.ClearProfileTexture(req.Context(), shared.CurrentActor(req), req.PathValue("pid"), "skin"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ClearProfileCape(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, textureClearOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.ClearProfileTexture(req.Context(), shared.CurrentActor(req), req.PathValue("pid"), "cape"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ListMyProfiles(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, profileReadOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.ListMyProfiles(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), limit)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}
