package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	invitesvc "element-skin/backend/internal/service/invite"
	"element-skin/backend/internal/util"
)

func (h Handler) Invites(w http.ResponseWriter, req *http.Request) {
	res, err := h.invites.List(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit"), 15))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) CreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	code, _ := body["code"].(string)
	note, _ := body["note"].(string)
	totalUses, totalUsesSet := body["total_uses"]
	res, err := h.invites.Create(req.Context(), shared.CurrentActor(req), invitesvc.CreateInput{
		Code:         code,
		TotalUses:    totalUses,
		TotalUsesSet: totalUsesSet,
		Note:         note,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, res)
}

func (h Handler) DeleteInvite(w http.ResponseWriter, req *http.Request) {
	if err := h.invites.Delete(req.Context(), shared.CurrentActor(req), req.PathValue("code")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
