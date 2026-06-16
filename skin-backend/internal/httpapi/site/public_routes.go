package site

import (
	"errors"
	"net/http"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (h Handler) PublicLibrary(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.PublicLibrary(req.Context(), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"), req.URL.Query().Get("q"), req.URL.Query().Get("sort"))
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

func (h Handler) PublicHomepageMedia(w http.ResponseWriter, req *http.Request) {
	if cached, err := h.redis.GetPublicHomepageMedia(req.Context()); err == nil {
		util.JSON(w, 200, cached)
		return
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	items, err := h.db.HomepageMedia.List(req.Context(), true)
	if err != nil {
		util.Error(w, err)
		return
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	if err := h.redis.SetPublicHomepageMedia(req.Context(), items, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, items)
}

func (h Handler) PublicFallbackStatus(w http.ResponseWriter, req *http.Request) {
	endpoints, err := h.db.Fallbacks.ListEndpoints(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	since := time.Now().Add(-redisstore.ProbeHistoryRetention)
	history, err := h.redis.GetProbeHistory(req.Context(), since)
	if err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, buildFallbackStatus(endpoints, history))
}

type fallbackStatusEntry struct {
	ID          int                 `json:"id"`
	Priority    int                 `json:"priority"`
	Note        string              `json:"note"`
	SessionURL  string              `json:"session_url"`
	AccountURL  string              `json:"account_url"`
	ServicesURL string              `json:"services_url"`
	Latest      *fallbackStatusTick `json:"latest"`
	History     []fallbackStatusTick `json:"history"`
}

type fallbackStatusTick struct {
	CheckedAt int64  `json:"checked_at"`
	Session   string `json:"session"`
	Account   string `json:"account"`
	Services  string `json:"services"`
}

func buildFallbackStatus(endpoints []map[string]any, history []redisstore.ProbeSample) map[string]any {
	byID := make(map[int][]redisstore.ProbeSample, len(endpoints))
	for _, sample := range history {
		byID[sample.EndpointID] = append(byID[sample.EndpointID], sample)
	}
	out := make([]fallbackStatusEntry, 0, len(endpoints))
	for _, ep := range endpoints {
		id, _ := ep["id"].(int)
		priority, _ := ep["priority"].(int)
		note, _ := ep["note"].(string)
		sessionURL, _ := ep["session_url"].(string)
		accountURL, _ := ep["account_url"].(string)
		servicesURL, _ := ep["services_url"].(string)
		samples := byID[id]
		ticks := make([]fallbackStatusTick, 0, len(samples))
		for _, sample := range samples {
			ticks = append(ticks, fallbackStatusTick{
				CheckedAt: sample.CheckedAt,
				Session:   sample.Session,
				Account:   sample.Account,
				Services:  sample.Services,
			})
		}
		var latest *fallbackStatusTick
		if len(ticks) > 0 {
			latest = &ticks[len(ticks)-1]
		}
		out = append(out, fallbackStatusEntry{
			ID:          id,
			Priority:    priority,
			Note:        note,
			SessionURL:  sessionURL,
			AccountURL:  accountURL,
			ServicesURL: servicesURL,
			Latest:      latest,
			History:     ticks,
		})
	}
	return map[string]any{
		"endpoints":     out,
		"retention_ms":  redisstore.ProbeHistoryRetention.Milliseconds(),
		"generated_at":  time.Now().UnixMilli(),
	}
}
