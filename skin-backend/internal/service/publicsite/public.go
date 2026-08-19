package publicsite

import (
	"context"
	"errors"
	"net/http"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

type Service struct {
	DB          *database.DB
	Redis       redisstore.Store
	Settings    settingssvc.Settings
	EmailPolicy emailpolicysvc.Service
	SiteURL     string
	APIURL      string
	CacheTTL    time.Duration
}

var sitePublicReadPermission = permission.MustDefinitionByCode("site_public.read.public")

func (s Service) PublicSettings(ctx context.Context, actor permission.Actor) (map[string]any, error) {
	if err := requirePublicPermission(actor); err != nil {
		return nil, err
	}
	if cached, err := s.Redis.GetPublicSettings(ctx); err == nil {
		if currentPublicSettingsCache(cached) {
			return cached, nil
		}
		if err := s.Redis.InvalidatePublicSettings(ctx); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	res, err := s.Settings.Public(ctx, s.SiteURL, s.APIURL)
	if err != nil {
		return nil, err
	}
	policy := s.EmailPolicy
	if policy.DB == nil {
		policy.DB = s.DB
	}
	publicPolicy, err := policy.Public(ctx)
	if err != nil {
		return nil, err
	}
	res["email_suffix_policy"] = publicPolicy
	if err := s.Redis.SetPublicSettings(ctx, res, s.CacheTTL); err != nil {
		return nil, err
	}
	return res, nil
}

func currentPublicSettingsCache(cached map[string]any) bool {
	_, hasInvite := cached["require_invite"]
	_, hasEmailSuffixPolicy := cached["email_suffix_policy"]
	return hasInvite && hasEmailSuffixPolicy
}

func (s Service) HomepageMedia(ctx context.Context, actor permission.Actor) ([]model.HomepageMedia, error) {
	if err := requirePublicPermission(actor); err != nil {
		return nil, err
	}
	if cached, err := s.Redis.GetPublicHomepageMedia(ctx); err == nil {
		return cached, nil
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	items, err := s.DB.HomepageMedia.List(ctx, true)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	if err := s.Redis.SetPublicHomepageMedia(ctx, items, s.CacheTTL); err != nil {
		return nil, err
	}
	return items, nil
}

func (s Service) FallbackStatus(ctx context.Context, actor permission.Actor, now time.Time) (map[string]any, error) {
	if err := requirePublicPermission(actor); err != nil {
		return nil, err
	}
	endpoints, err := s.DB.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	since := now.Add(-redisstore.ProbeHistoryRetention)
	history, err := s.Redis.GetProbeHistory(ctx, since)
	if err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	return buildFallbackStatus(endpoints, history, now), nil
}

func requirePublicPermission(actor permission.Actor) error {
	if err := actor.Require(sitePublicReadPermission); err != nil {
		return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
	}
	return nil
}

type fallbackStatusEntry struct {
	ID          int                  `json:"id"`
	Priority    int                  `json:"priority"`
	Note        string               `json:"note"`
	SessionURL  string               `json:"session_url"`
	AccountURL  string               `json:"account_url"`
	ServicesURL string               `json:"services_url"`
	Latest      *fallbackStatusTick  `json:"latest"`
	History     []fallbackStatusTick `json:"history"`
}

type fallbackStatusTick struct {
	CheckedAt int64  `json:"checked_at"`
	Session   string `json:"session"`
	Account   string `json:"account"`
	Services  string `json:"services"`
}

func buildFallbackStatus(endpoints []map[string]any, history []redisstore.ProbeSample, now time.Time) map[string]any {
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
		"endpoints":    out,
		"retention_ms": redisstore.ProbeHistoryRetention.Milliseconds(),
		"generated_at": now.UnixMilli(),
	}
}
