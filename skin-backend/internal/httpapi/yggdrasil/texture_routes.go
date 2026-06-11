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
	textureType := strings.ToLower(req.PathValue("texture_type"))
	if textureType != "skin" && textureType != "cape" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
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
	storage, err := texturesvc.NewTextureStorage(h.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, created, err := storage.ProcessAndSaveTracked(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	if err := h.db.Textures.AddToLibrary(req.Context(), tok.UserID, hash, textureType, "", false, profilestore.NormalizeModel(req.FormValue("model"))); err != nil {
		if created {
			if inUse, checkErr := h.db.Textures.ExistsHash(req.Context(), hash); checkErr == nil && !inUse {
				_ = storage.DeleteFile(hash)
			}
		}
		util.Error(w, err)
		return
	}
	if textureType == "skin" {
		if err := h.site.SetProfileTexture(req.Context(), selectedProfile.ID, "skin", &hash); err != nil {
			util.Error(w, err)
			return
		}
		_ = h.db.Profiles.UpdateModel(req.Context(), selectedProfile.ID, profilestore.NormalizeModel(req.FormValue("model")))
	} else if textureType == "cape" {
		if err := h.site.SetProfileTexture(req.Context(), selectedProfile.ID, "cape", &hash); err != nil {
			util.Error(w, err)
			return
		}
	}
	w.WriteHeader(204)
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
		err = h.site.SetProfileTexture(req.Context(), *tok.ProfileID, "skin", nil)
	case "cape":
		err = h.site.SetProfileTexture(req.Context(), *tok.ProfileID, "cape", nil)
	default:
		err = util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}
