package yggdrasil

import (
	"context"
	"net/http"
	"strings"

	permissiondb "element-skin/backend/internal/database/permission"
	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
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
	actor, err := h.yggTextureActor(req.Context(), tok)
	if err != nil {
		util.Error(w, err)
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
	if err := h.site.ApplyTextureToProfileWithModel(
		req.Context(),
		actor,
		selectedProfile.ID,
		hash,
		textureType,
		profilestore.NormalizeModel(req.FormValue("model")),
	); err != nil {
		util.Error(w, err)
		return
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
	actor, err := h.yggTextureActor(req.Context(), tok)
	if err != nil {
		util.Error(w, err)
		return
	}
	switch strings.ToLower(req.PathValue("texture_type")) {
	case "skin":
		err = h.site.ClearProfileTexture(req.Context(), actor, *tok.ProfileID, "skin")
	case "cape":
		err = h.site.ClearProfileTexture(req.Context(), actor, *tok.ProfileID, "cape")
	default:
		err = util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h Handler) yggTextureActor(ctx context.Context, token model.Token) (permission.Actor, error) {
	actor, err := h.db.Permissions.ActorForUser(ctx, token.UserID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindYggdrasil,
		Entrypoint:  permission.EntrypointYggdrasil,
	})
	if err != nil {
		return permission.Actor{}, err
	}
	if token.ProfileID != nil {
		actor.BoundProfileID = *token.ProfileID
	}
	return actor, nil
}
