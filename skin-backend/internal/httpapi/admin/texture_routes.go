package admin

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Textures(w http.ResponseWriter, req *http.Request) {
	res, err := h.textures.ListAllTextures(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit")), req.URL.Query().Get("q"), req.URL.Query().Get("type"))
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
	hash := req.PathValue("hash")
	textureType := textureTypeFromRequest(req, body)
	if err := h.textures.UpdateAnyTexture(req.Context(), shared.CurrentActor(req), hash, textureType, body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	force := req.URL.Query().Get("force") == "true"
	typ := req.URL.Query().Get("type")
	if err := h.textures.DeleteAnyTexture(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), typ, req.URL.Query().Get("user_id"), force); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"success": true})
}

func textureTypeFromRequest(req *http.Request, body map[string]any) string {
	textureType := strings.TrimSpace(req.URL.Query().Get("type"))
	if textureType != "" {
		return textureType
	}
	if v, ok := body["type"].(string); ok {
		return strings.TrimSpace(v)
	}
	return "skin"
}
