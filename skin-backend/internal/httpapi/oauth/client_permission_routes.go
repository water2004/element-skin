package oauth

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) ClientPermissions(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ClientPermissions(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) SetClientPermission(w http.ResponseWriter, req *http.Request) {
	var body permissionOverrideBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.oauth.SetClientPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), req.PathValue("permission_code"), body.Effect); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ClearClientPermission(w http.ResponseWriter, req *http.Request) {
	if err := h.oauth.ClearClientPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), req.PathValue("permission_code")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

type permissionOverrideBody struct {
	Effect string `json:"effect"`
}
