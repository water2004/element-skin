package notice

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/util"
)

func (h Handler) List(w http.ResponseWriter, req *http.Request) {
	res, err := h.notices.ListForUser(req.Context(), shared.CurrentActor(req), noticesvc.ListParams{
		Type:        req.URL.Query().Get("type"),
		Limit:       util.ClampLimit(req.URL.Query().Get("limit")),
		Cursor:      req.URL.Query().Get("cursor"),
		IncludeRead: req.URL.Query().Get("include_read") != "false",
		Dashboard:   req.URL.Query().Get("dashboard") == "true",
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) Detail(w http.ResponseWriter, req *http.Request) {
	res, err := h.notices.GetForUser(req.Context(), req.PathValue("id"), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) MarkRead(w http.ResponseWriter, req *http.Request) {
	if err := h.notices.MarkRead(req.Context(), req.PathValue("id"), shared.CurrentActor(req)); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) Dismiss(w http.ResponseWriter, req *http.Request) {
	if err := h.notices.Dismiss(req.Context(), req.PathValue("id"), shared.CurrentActor(req)); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
