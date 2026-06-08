package site

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

func (h Handler) PublicLibrary(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.PublicLibrary(req.Context(), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"), req.URL.Query().Get("q"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (settingssvc.Settings{DB: h.db}).Public(req.Context(), h.cfg.SiteURL, h.cfg.APIURL)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicCarousel(w http.ResponseWriter, req *http.Request) {
	entries, err := os.ReadDir(h.cfg.CarouselDir)
	if os.IsNotExist(err) {
		util.JSON(w, 200, []string{})
		return
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch strings.ToLower(filepath.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".webp":
			images = append(images, name)
		}
	}
	if images == nil {
		images = []string{}
	}
	util.JSON(w, 200, images)
}
