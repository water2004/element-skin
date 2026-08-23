package remote

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) GetProfiles(w http.ResponseWriter, req *http.Request) {
	var body struct {
		APIURL   string `json:"api_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	profiles, err := h.imports.PreviewProfiles(req.Context(), shared.CurrentActor(req), body.APIURL, body.Username, body.Password)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": profiles})
}

func (h Handler) ImportProfiles(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	profiles, err := shared.ParseImportProfiles(body["profiles"])
	if err != nil {
		util.Error(w, err)
		return
	}
	res, err := h.imports.ImportProfiles(req.Context(), shared.CurrentActor(req), shared.AsString(body["api_url"]), profiles)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) ImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	profileID := strings.TrimSpace(body["profile_id"])
	profileName := strings.TrimSpace(body["profile_name"])
	if profileID == "" || profileName == "" {
		util.Error(w, util.HTTPError{Status: 400, Object: "profile", Operation: "import", Reason: "required"})
		return
	}
	res, err := h.imports.ImportProfile(req.Context(), shared.CurrentActor(req), body["api_url"], profileID, profileName)
	if err != nil {
		util.Error(w, err)
		return
	}
	profile := res["profile"].(map[string]any)
	util.JSON(w, http.StatusCreated, map[string]any{"id": profile["id"], "name": profile["name"]})
}
