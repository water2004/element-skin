package yggdrasil

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Metadata(w http.ResponseWriter, req *http.Request) {
	res, err := h.ygg.Metadata(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) Authenticate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		ClientToken string `json:"clientToken"`
		RequestUser bool   `json:"requestUser"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.ygg.Authenticate(req.Context(), body.Username, body.Password, body.ClientToken, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) Refresh(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AccessToken     string         `json:"accessToken"`
		ClientToken     string         `json:"clientToken"`
		RequestUser     bool           `json:"requestUser"`
		SelectedProfile map[string]any `json:"selectedProfile"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	selected := ""
	if body.SelectedProfile != nil {
		selected, _ = body.SelectedProfile["id"].(string)
	}
	res, err := h.ygg.Refresh(req.Context(), body.AccessToken, body.ClientToken, selected, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) Validate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.ygg.Validate(req.Context(), body["accessToken"], body["clientToken"]); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h Handler) Invalidate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if body["accessToken"] != "" {
		_ = h.db.DeleteToken(req.Context(), body["accessToken"])
	}
	w.WriteHeader(204)
}

func (h Handler) Signout(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(204)
}

func (h Handler) Join(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.ygg.Join(req.Context(), body["accessToken"], body["selectedProfile"], body["serverId"], req.RemoteAddr); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}
