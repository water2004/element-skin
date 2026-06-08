package site

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"element-skin/backend/internal/redisstore"
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
	if cached, err := h.redis.GetPublicSettings(req.Context()); err == nil {
		util.JSON(w, 200, cached)
		return
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	res, err := h.settings.Public(req.Context(), h.cfg.SiteURL, h.cfg.APIURL)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.SetPublicSettings(req.Context(), res, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicCarousel(w http.ResponseWriter, req *http.Request) {
	if cached, err := h.redis.GetPublicCarousel(req.Context()); err == nil {
		util.JSON(w, 200, cached)
		return
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	entries, err := os.ReadDir(h.cfg.CarouselDir)
	if os.IsNotExist(err) {
		images := []string{}
		if err := h.redis.SetPublicCarousel(req.Context(), images, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
			util.Error(w, err)
			return
		}
		util.JSON(w, 200, images)
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
	if err := h.redis.SetPublicCarousel(req.Context(), images, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, images)
}
