package minecraft

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	minecraftsvc "element-skin/backend/internal/service/minecraft"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

type Handler struct {
	auth      shared.AuthFunc
	minecraft minecraftsvc.Service
}

func New(db *database.DB, auth shared.AuthFunc, ygg yggsvc.Yggdrasil) Handler {
	return Handler{
		auth: auth,
		minecraft: minecraftsvc.Service{
			DB:  db,
			Ygg: ygg,
		},
	}
}

func (h Handler) Auth(next http.HandlerFunc, required ...permission.Definition) http.HandlerFunc {
	return h.auth(next, required...)
}

func (h Handler) ProfileByName(w http.ResponseWriter, req *http.Request) {
	res, err := h.minecraft.ProfileByName(req.Context(), shared.CurrentActor(req), req.PathValue("name"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) Profiles(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(strings.Trim(req.PathValue("path"), "/"), "/")
	if len(parts) == 2 && parts[0] == "by-name" {
		req.SetPathValue("name", parts[1])
		h.ProfileByName(w, req)
		return
	}
	if len(parts) == 1 && parts[0] != "" {
		req.SetPathValue("profile_id", parts[0])
		h.ProfileByID(w, req)
		return
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "textures-property" {
		req.SetPathValue("profile_id", parts[0])
		h.TexturesProperty(w, req)
		return
	}
	util.Error(w, util.HTTPError{Status: http.StatusNotFound, Object: "minecraft_route", Operation: "resolve", Reason: "not_found"})
}

func (h Handler) ProfilesByNames(w http.ResponseWriter, req *http.Request) {
	var body namesBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	res, err := h.minecraft.ProfilesByNames(req.Context(), shared.CurrentActor(req), body.Names)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) ProfileByID(w http.ResponseWriter, req *http.Request) {
	res, err := h.minecraft.ProfileByID(req.Context(), shared.CurrentActor(req), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) TexturesProperty(w http.ResponseWriter, req *http.Request) {
	res, err := h.minecraft.TexturesProperty(req.Context(), shared.CurrentActor(req), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) HasJoined(w http.ResponseWriter, req *http.Request) {
	var body hasJoinedBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	res, err := h.minecraft.HasJoined(req.Context(), shared.CurrentActor(req), minecraftsvc.HasJoinedRequest{
		Username: body.Username,
		ServerID: body.ServerID,
		IP:       body.IP,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

type namesBody struct {
	Names []string `json:"names"`
}

type hasJoinedBody struct {
	Username string `json:"username"`
	ServerID string `json:"server_id"`
	IP       string `json:"ip"`
}
