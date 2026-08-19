package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Profiles(w http.ResponseWriter, req *http.Request) {
	res, err := h.profiles.ListAllProfiles(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit")), req.URL.Query().Get("q"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if err := h.profiles.UpdateAnyProfile(req.Context(), shared.CurrentActor(req), req.PathValue("profile_id"), body["name"]); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) DeleteProfile(w http.ResponseWriter, req *http.Request) {
	err := h.profiles.DeleteProfileByID(req.Context(), shared.CurrentActor(req), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) UpdateProfileSkin(w http.ResponseWriter, req *http.Request) {
	h.setProfileTexture(w, req, "skin")
}

func (h Handler) UpdateProfileCape(w http.ResponseWriter, req *http.Request) {
	h.setProfileTexture(w, req, "cape")
}

func (h Handler) setProfileTexture(w http.ResponseWriter, req *http.Request, typ string) {
	var body map[string]*string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	profileID := req.PathValue("profile_id")
	if err := h.profiles.SetProfileTexture(req.Context(), shared.CurrentActor(req), profileID, typ, body["hash"]); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
