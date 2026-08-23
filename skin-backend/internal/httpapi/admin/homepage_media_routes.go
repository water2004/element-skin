package admin

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/util"
)

func (h Handler) ListHomepageMedia(w http.ResponseWriter, req *http.Request) {
	items, err := h.homepage.List(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) UploadHomepageImage(w http.ResponseWriter, req *http.Request) {
	upload, err := shared.ReadMultipartUpload(req, "file", homepagesvc.MaxImageBytes)
	if err != nil {
		util.Error(w, err)
		return
	}
	item, err := h.homepage.UploadImage(req.Context(), shared.CurrentActor(req), homepagesvc.UploadInput{
		Filename: upload.Filename,
		Data:     upload.Data,
		Fields:   upload.Fields,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, item)
}

func (h Handler) UploadHomepagePanorama(w http.ResponseWriter, req *http.Request) {
	upload, err := shared.ReadMultipartUpload(req, "file", homepagesvc.MaxPanoramaBytes)
	if err != nil {
		util.Error(w, err)
		return
	}
	item, err := h.homepage.UploadPanorama(req.Context(), shared.CurrentActor(req), homepagesvc.UploadInput{
		Filename: upload.Filename,
		Data:     upload.Data,
		Fields:   upload.Fields,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, item)
}

func (h Handler) PatchHomepageMedia(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Title               *string  `json:"title"`
		OverlayOpacityLight *float64 `json:"overlay_opacity_light"`
		OverlayOpacityDark  *float64 `json:"overlay_opacity_dark"`
		StartYaw            *float64 `json:"start_yaw"`
		StartPitch          *float64 `json:"start_pitch"`
		YawSpeedDPS         *float64 `json:"yaw_speed_dps"`
		PitchSpeedDPS       *float64 `json:"pitch_speed_dps"`
		Enabled             *bool    `json:"enabled"`
		DurationMS          *int     `json:"duration_ms"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	item, err := h.homepage.Patch(req.Context(), shared.CurrentActor(req), req.PathValue("id"), homepagesvc.PatchInput{
		Title:               body.Title,
		OverlayOpacityLight: body.OverlayOpacityLight,
		OverlayOpacityDark:  body.OverlayOpacityDark,
		StartYaw:            body.StartYaw,
		StartPitch:          body.StartPitch,
		YawSpeedDPS:         body.YawSpeedDPS,
		PitchSpeedDPS:       body.PitchSpeedDPS,
		Enabled:             body.Enabled,
		DurationMS:          body.DurationMS,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, item)
}

func (h Handler) ReorderHomepageMedia(w http.ResponseWriter, req *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if err := h.homepage.Reorder(req.Context(), shared.CurrentActor(req), body.IDs); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) DeleteHomepageMedia(w http.ResponseWriter, req *http.Request) {
	if err := h.homepage.Delete(req.Context(), shared.CurrentActor(req), req.PathValue("id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
