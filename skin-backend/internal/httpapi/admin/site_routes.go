package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

func (h Handler) Invites(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastCode, err := shared.CursorCreatedHash(req.URL.Query().Get("cursor"), "last_code")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.ListInvites(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), lastCreated, lastCode)
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) CreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	code, _ := body["code"].(string)
	if code == "" {
		id, err := util.GenerateUUIDNoDash()
		if err != nil {
			util.Error(w, err)
			return
		}
		code = id + id[:8]
	}
	if len(code) < 4 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invite code too short"})
		return
	}
	total := 1
	if v, ok := body["total_uses"].(float64); ok {
		total = int(v)
	}
	note, _ := body["note"].(string)
	if err := h.db.CreateInvite(req.Context(), code, total, note); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"code": code, "total_uses": total, "note": note})
}

func (h Handler) DeleteInvite(w http.ResponseWriter, req *http.Request) {
	if err := h.db.DeleteInvite(req.Context(), req.PathValue("code")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) OfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := shared.ParsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	users, err := h.db.ListWhitelistUsers(req.Context(), endpointID)
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
	if err := h.db.AddWhitelistUser(req.Context(), username, endpointID); err != nil {
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
	if err := h.db.RemoveWhitelistUser(req.Context(), req.PathValue("username"), endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) UploadCarousel(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(6 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "file is required"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		util.Error(w, util.HTTPError{Status: 400, Detail: "Unsupported file format"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > 5*1024*1024 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	if err := os.MkdirAll(h.cfg.CarouselDir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		util.Error(w, err)
		return
	}
	filename := id + ext
	if err := os.WriteFile(filepath.Join(h.cfg.CarouselDir, filename), data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"filename": filename})
}

func (h Handler) DeleteCarousel(w http.ResponseWriter, req *http.Request) {
	filename := filepath.Base(req.PathValue("filename"))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid filename"})
		return
	}
	err := os.Remove(filepath.Join(h.cfg.CarouselDir, filename))
	if err != nil && !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) GetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (settingssvc.Settings{DB: h.db}).GetGroup(req.Context(), "site")
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SaveSiteSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (settingssvc.Settings{DB: h.db}).SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) GetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := (settingssvc.Settings{DB: h.db}).GetGroup(req.Context(), req.PathValue("group"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SaveSettingsGroup(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (settingssvc.Settings{DB: h.db}).SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
