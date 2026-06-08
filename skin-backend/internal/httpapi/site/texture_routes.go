package site

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/httpapi/shared"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

func (h Handler) ListMyTextures(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.ListMyTextures(req.Context(), shared.CurrentUserID(req), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UploadMyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := shared.MultipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if textureType == "" {
		textureType = "skin"
	}
	if textureType != "skin" && textureType != "cape" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
	storage, err := texturesvc.NewTextureStorage(h.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	if err := h.db.Textures.AddToLibrary(req.Context(), shared.CurrentUserID(req), hash, textureType, req.FormValue("note"), shared.FormBool(req.FormValue("is_public")), profile.NormalizeModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash, "texture_type": textureType})
}

func (h Handler) UploadAndApplyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	profileID := strings.TrimSpace(req.FormValue("uuid"))
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if profileID == "" || textureType == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "uuid and texture_type are required"})
		return
	}
	data, err := shared.MultipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	storage, err := texturesvc.NewTextureStorage(h.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	model := profile.NormalizeModel(req.FormValue("model"))
	if err := h.db.Textures.AddToLibrary(req.Context(), shared.CurrentUserID(req), hash, textureType, "", shared.FormBool(req.FormValue("is_public")), model); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.ApplyTextureToProfile(req.Context(), shared.CurrentUserID(req), profileID, hash, textureType); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "hash": hash, "type": textureType})
}

func (h Handler) TextureDetail(w http.ResponseWriter, req *http.Request) {
	res, err := h.site.TextureDetail(req.Context(), shared.CurrentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.site.UpdateTexture(req.Context(), shared.CurrentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	if err := h.site.DeleteTexture(req.Context(), shared.CurrentUserID(req), req.PathValue("hash"), req.PathValue("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) AddTexture(w http.ResponseWriter, req *http.Request) {
	if err := h.site.AddTextureToWardrobe(req.Context(), shared.CurrentUserID(req), req.PathValue("hash"), req.URL.Query().Get("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ApplyTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.site.ApplyTextureToProfile(req.Context(), shared.CurrentUserID(req), body["profile_id"], req.PathValue("hash"), body["texture_type"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
