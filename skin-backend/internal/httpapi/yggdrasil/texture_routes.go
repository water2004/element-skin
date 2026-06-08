package yggdrasil

import (
	"net/http"
	"strings"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/httpapi/shared"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

func (h Handler) UploadTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := shared.BearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := h.ygg.Token(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	selectedProfile, err := h.db.Profiles.GetByID(req.Context(), *tok.ProfileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if selectedProfile == nil || selectedProfile.UserID != tok.UserID {
		util.Error(w, util.HTTPError{Status: 403, Detail: "Profile not yours"})
		return
	}
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := shared.MultipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(req.PathValue("texture_type"))
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
	if err := h.db.Textures.AddToLibrary(req.Context(), tok.UserID, hash, textureType, "", false, profilestore.NormalizeModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	if textureType == "skin" {
		if err := h.db.Profiles.UpdateSkin(req.Context(), selectedProfile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
		_ = h.db.Profiles.UpdateModel(req.Context(), selectedProfile.ID, profilestore.NormalizeModel(req.FormValue("model")))
	} else if textureType == "cape" {
		if err := h.db.Profiles.UpdateCape(req.Context(), selectedProfile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
	} else {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash})
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := shared.BearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := h.ygg.Token(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	switch strings.ToLower(req.PathValue("texture_type")) {
	case "skin":
		err = h.db.Profiles.UpdateSkin(req.Context(), *tok.ProfileID, nil)
	case "cape":
		err = h.db.Profiles.UpdateCape(req.Context(), *tok.ProfileID, nil)
	default:
		err = util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}
