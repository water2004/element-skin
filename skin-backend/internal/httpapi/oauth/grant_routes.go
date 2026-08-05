package oauth

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) ListGrants(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ListGrants(req.Context(), shared.CurrentActor(req), util.ClampLimit(req.URL.Query().Get("limit")))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h Handler) RevokeGrant(w http.ResponseWriter, req *http.Request) {
	if err := h.oauth.RevokeGrant(req.Context(), shared.CurrentActor(req), req.PathValue("grant_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
