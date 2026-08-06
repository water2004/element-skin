package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func (h Handler) GetEmailSuffixPolicy(w http.ResponseWriter, req *http.Request) {
	policy, err := h.emailPolicy.Read(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, policy)
}

func (h Handler) PutEmailSuffixPolicy(w http.ResponseWriter, req *http.Request) {
	var body model.EmailSuffixPolicy
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid json"})
		return
	}
	if err := h.emailPolicy.Update(req.Context(), shared.CurrentActor(req), body); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
