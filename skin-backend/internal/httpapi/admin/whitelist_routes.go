package admin

import (
	"fmt"
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/util"
)

func (h Handler) OfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, _ := shared.ParsePositiveInt(req.URL.Query().Get("endpoint_id"))
	users, err := h.fallback.ListWhitelistUsers(req.Context(), shared.CurrentActor(req), endpointID)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"items": users})
}

func (h Handler) AddOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	endpointID, _ := shared.ParsePositiveInt(fmt.Sprint(body["endpoint_id"]))
	if err := h.fallback.AddWhitelistUser(req.Context(), shared.CurrentActor(req), fallbacksvc.WhitelistInput{
		Username:   shared.AsString(body["username"]),
		EndpointID: endpointID,
	}); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, map[string]any{
		"username":    shared.AsString(body["username"]),
		"endpoint_id": endpointID,
	})
}

func (h Handler) RemoveOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, _ := shared.ParsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err := h.fallback.RemoveWhitelistUser(req.Context(), shared.CurrentActor(req), req.PathValue("username"), endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
