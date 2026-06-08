package admin

import (
	"fmt"
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) OfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := shared.ParsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	users, err := h.db.Fallbacks.ListWhitelistUsers(req.Context(), endpointID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if users == nil {
		users = []map[string]any{}
	}
	util.JSON(w, 200, map[string]any{"items": users})
}

func (h Handler) AddOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	username := strings.TrimSpace(shared.AsString(body["username"]))
	endpointID, err := shared.ParsePositiveInt(fmt.Sprint(body["endpoint_id"]))
	if username == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "username is required"})
		return
	}
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := h.db.Fallbacks.AddWhitelistUser(req.Context(), username, endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) RemoveOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := shared.ParsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := h.db.Fallbacks.RemoveWhitelistUser(req.Context(), req.PathValue("username"), endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
