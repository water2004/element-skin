package yggdrasil

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/util"
)

func (h Handler) HasJoined(w http.ResponseWriter, req *http.Request) {
	username := req.URL.Query().Get("username")
	serverID := req.URL.Query().Get("serverId")
	res, status, err := h.ygg.HasJoined(req.Context(), username, serverID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := h.fallback.HasJoined(req.Context(), username, serverID, req.URL.Query().Get("ip"))
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, status, res)
}

func (h Handler) Profile(w http.ResponseWriter, req *http.Request) {
	unsigned := req.URL.Query().Get("unsigned") != "false"
	res, status, err := h.ygg.Profile(req.Context(), req.PathValue("uuid"), unsigned)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := h.fallback.GetProfile(req.Context(), req.PathValue("uuid"), unsigned)
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func writeFallback(w http.ResponseWriter, resp *fallbacksvc.FallbackResponse) bool {
	if resp == nil {
		return false
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
	return true
}

func (h Handler) LookupName(w http.ResponseWriter, req *http.Request) {
	playerName := req.PathValue("playerName")
	res, status, err := h.ygg.LookupName(req.Context(), playerName)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		var resp *fallbacksvc.FallbackResponse
		if strings.HasPrefix(req.URL.Path, "/api/minecraft/profile/lookup/name/") || strings.HasPrefix(req.URL.Path, "/minecraft/profile/lookup/name/") {
			resp, err = h.fallback.ServicesLookup(req.Context(), playerName)
		} else {
			resp, err = h.fallback.GetProfileByName(req.Context(), playerName)
		}
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) LookupNames(w http.ResponseWriter, req *http.Request) {
	var names []string
	if err := shared.DecodeJSON(req, &names); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	profiles, err := h.fallback.LookupNames(req.Context(), names)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, profiles)
}
