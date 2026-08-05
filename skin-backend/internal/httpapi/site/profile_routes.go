package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) CreateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.profiles.CreateProfile(req.Context(), shared.CurrentActor(req), body["name"], body["model"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, res)
}

func (h Handler) UpdateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.profiles.UpdateProfile(req.Context(), shared.CurrentActor(req), profilePathID(req), body["name"]); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) DeleteProfile(w http.ResponseWriter, req *http.Request) {
	if err := h.profiles.DeleteProfile(req.Context(), shared.CurrentActor(req), profilePathID(req)); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ClearProfileSkin(w http.ResponseWriter, req *http.Request) {
	if err := h.profiles.ClearProfileTexture(req.Context(), shared.CurrentActor(req), profilePathID(req), "skin"); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ClearProfileCape(w http.ResponseWriter, req *http.Request) {
	if err := h.profiles.ClearProfileTexture(req.Context(), shared.CurrentActor(req), profilePathID(req), "cape"); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func profilePathID(req *http.Request) string {
	if id := req.PathValue("profile_id"); id != "" {
		return id
	}
	return req.PathValue("pid")
}

func (h Handler) ListMyProfiles(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.profiles.ListMyProfiles(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), limit)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}
