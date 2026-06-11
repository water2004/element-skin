package admin

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Profiles(w http.ResponseWriter, req *http.Request) {
	rawCursor := req.URL.Query().Get("cursor")
	cursor, err := util.DecodeCursor(rawCursor)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	if rawCursor != "" && last == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Profiles.ListAll(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) UpdateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	p, err := h.db.Profiles.GetByID(req.Context(), profileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if body["name"] != "" {
		if !util.ValidProfileName(body["name"]) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid profile name"})
			return
		}
		ok, err := h.db.Profiles.UpdateName(req.Context(), profileID, body["name"])
		if err != nil {
			if profile.IsNameConflict(err) {
				util.Error(w, util.HTTPError{Status: 409, Detail: "profile name already exists"})
				return
			}
			util.Error(w, err)
			return
		}
		if !ok {
			util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteProfile(w http.ResponseWriter, req *http.Request) {
	err := h.site.DeleteProfileByID(req.Context(), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
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
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	if p, err := h.db.Profiles.GetByID(req.Context(), profileID); err != nil {
		util.Error(w, err)
		return
	} else if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if err := h.site.SetProfileTexture(req.Context(), profileID, typ, body["hash"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
