package officialprofile

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) List(w http.ResponseWriter, req *http.Request) {
	items, err := h.service.List(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) Create(w http.ResponseWriter, req *http.Request) {
	var body struct {
		IdentityID string `json:"identity_id"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid json"})
		return
	}
	item, err := h.service.Create(req.Context(), shared.CurrentActor(req), body.IdentityID)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, item)
}

func (h Handler) Sync(w http.ResponseWriter, req *http.Request) {
	item, err := h.service.Sync(req.Context(), shared.CurrentActor(req), req.PathValue("binding_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, item)
}

func (h Handler) Delete(w http.ResponseWriter, req *http.Request) {
	if err := h.service.Delete(req.Context(), shared.CurrentActor(req), req.PathValue("binding_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
