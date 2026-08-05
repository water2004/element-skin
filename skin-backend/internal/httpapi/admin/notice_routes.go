package admin

import (
	"bytes"
	"encoding/json"
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/util"
)

func (h Handler) Notices(w http.ResponseWriter, req *http.Request) {
	res, err := h.notices.ListForManagement(req.Context(), shared.CurrentActor(req), noticesvc.ListParams{
		Type:   req.URL.Query().Get("type"),
		Status: req.URL.Query().Get("status"),
		Limit:  util.ClampLimit(req.URL.Query().Get("limit")),
		Cursor: req.URL.Query().Get("cursor"),
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) CreateNotice(w http.ResponseWriter, req *http.Request) {
	var body noticesvc.CreateInput
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid json"})
		return
	}
	res, err := h.notices.Create(req.Context(), shared.CurrentActor(req), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, res)
}

func (h Handler) PatchNotice(w http.ResponseWriter, req *http.Request) {
	var raw map[string]json.RawMessage
	if err := shared.DecodeJSON(req, &raw); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid json"})
		return
	}
	body, err := noticePatchInput(raw)
	if err != nil {
		util.Error(w, err)
		return
	}
	res, err := h.notices.Patch(req.Context(), shared.CurrentActor(req), req.PathValue("id"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) DeleteNotice(w http.ResponseWriter, req *http.Request) {
	if err := h.notices.Delete(req.Context(), shared.CurrentActor(req), req.PathValue("id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func noticePatchInput(raw map[string]json.RawMessage) (noticesvc.PatchInput, error) {
	var out noticesvc.PatchInput
	for key, value := range raw {
		switch key {
		case "type":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.Type = &v
		case "title":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.Title = &v
		case "summary":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.Summary = &v
		case "content_markdown":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.ContentMarkdown = &v
		case "display_mode":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.DisplayMode = &v
		case "level":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.Level = &v
		case "link_text":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.LinkText = &v
		case "link_url":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.LinkURL = &v
		case "audience":
			v, err := patchString(value)
			if err != nil {
				return out, err
			}
			out.Audience = &v
		case "enabled":
			v, err := patchBool(value)
			if err != nil {
				return out, err
			}
			out.Enabled = &v
		case "pinned":
			v, err := patchBool(value)
			if err != nil {
				return out, err
			}
			out.Pinned = &v
		case "dismissible":
			v, err := patchBool(value)
			if err != nil {
				return out, err
			}
			out.Dismissible = &v
		case "starts_at":
			v, clear, err := patchOptionalInt64(value)
			if err != nil {
				return out, err
			}
			out.StartsAt = v
			out.ClearStartsAt = clear
		case "ends_at":
			v, clear, err := patchOptionalInt64(value)
			if err != nil {
				return out, err
			}
			out.EndsAt = v
			out.ClearEndsAt = clear
		}
	}
	return out, nil
}

func patchString(raw json.RawMessage) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid patch value"}
	}
	return value, nil
}

func patchBool(raw json.RawMessage) (bool, error) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid patch value"}
	}
	return value, nil
}

func patchOptionalInt64(raw json.RawMessage) (*int64, bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true, nil
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid patch value"}
	}
	return &value, false, nil
}
