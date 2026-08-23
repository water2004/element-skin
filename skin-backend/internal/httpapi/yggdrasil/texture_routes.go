package yggdrasil

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

func (h Handler) UploadTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := shared.BearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"})
		return
	}
	tok, err := h.ygg.Token(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Object: "access_token", Operation: "verify", Reason: "invalid"})
		return
	}
	actor, err := h.ygg.ActorForToken(req.Context(), tok, false)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(req.PathValue("texture_type"))
	if textureType != "skin" && textureType != "cape" {
		util.Error(w, util.HTTPError{Status: 400, Object: "texture_type", Operation: "validate", Reason: "invalid"})
		return
	}
	upload, err := shared.ReadMultipartUpload(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	if _, err := h.uploads.UploadAndApplyBoundProfile(
		req.Context(),
		texturesvc.UploadInput{
			Actor:       actor,
			Data:        upload.Data,
			TextureType: textureType,
			Model:       upload.Fields["model"],
		},
		*tok.ProfileID,
	); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := shared.BearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"})
		return
	}
	tok, err := h.ygg.Token(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Object: "access_token", Operation: "verify", Reason: "invalid"})
		return
	}
	actor, err := h.ygg.ActorForToken(req.Context(), tok, false)
	if err != nil {
		util.Error(w, err)
		return
	}
	switch strings.ToLower(req.PathValue("texture_type")) {
	case "skin":
		err = h.profiles.ClearProfileTexture(req.Context(), actor, *tok.ProfileID, "skin")
	case "cape":
		err = h.profiles.ClearProfileTexture(req.Context(), actor, *tok.ProfileID, "cape")
	default:
		err = util.HTTPError{Status: 400, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}
