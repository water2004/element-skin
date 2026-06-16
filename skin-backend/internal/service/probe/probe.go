package probe

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
)

const (
	TestUUID    = "853c80ef3c3749fdaa49938b674adae6"
	TestName    = "jeb_"
	StatusUp    = "up"
	StatusDown  = "down"
	httpTimeout = 8 * time.Second
)

type Service struct {
	DB        *database.DB
	Redis     redisstore.Store
	Client    *http.Client
	Now       func() time.Time
	Retention time.Duration
}

func New(db *database.DB, redis redisstore.Store) *Service {
	return &Service{
		DB:        db,
		Redis:     redis,
		Client:    &http.Client{Timeout: httpTimeout},
		Now:       time.Now,
		Retention: redisstore.ProbeHistoryRetention,
	}
}

func (s *Service) Run(ctx context.Context) error {
	endpoints, err := s.DB.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		return err
	}
	samples := make([]redisstore.ProbeSample, len(endpoints))
	var wg sync.WaitGroup
	for i, ep := range endpoints {
		wg.Add(1)
		go func(idx int, ep map[string]any) {
			defer wg.Done()
			samples[idx] = s.probeEndpoint(ctx, ep)
		}(i, ep)
	}
	wg.Wait()
	return s.Redis.AppendProbeSamples(ctx, samples, s.retention())
}

func (s *Service) probeEndpoint(ctx context.Context, ep map[string]any) redisstore.ProbeSample {
	id, _ := ep["id"].(int)
	note, _ := ep["note"].(string)
	sessionURL, _ := ep["session_url"].(string)
	accountURL, _ := ep["account_url"].(string)
	servicesURL, _ := ep["services_url"].(string)
	return redisstore.ProbeSample{
		EndpointID:  id,
		Note:        note,
		SessionURL:  sessionURL,
		AccountURL:  accountURL,
		ServicesURL: servicesURL,
		CheckedAt:   s.now().UnixMilli(),
		Session:     s.checkURL(ctx, joinURL(sessionURL, "/session/minecraft/profile/"+TestUUID)),
		Account:     s.checkURL(ctx, joinURL(accountURL, "/users/profiles/minecraft/"+TestName)),
		Services:    s.checkURL(ctx, joinURL(servicesURL, "/minecraft/profile/lookup/name/"+TestName)),
	}
}

func (s *Service) checkURL(ctx context.Context, rawURL string) string {
	if rawURL == "" {
		return StatusDown
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return StatusDown
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return StatusDown
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return StatusUp
	}
	return StatusDown
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) retention() time.Duration {
	if s.Retention > 0 {
		return s.Retention
	}
	return redisstore.ProbeHistoryRetention
}

func joinURL(base, path string) string {
	if base == "" {
		return ""
	}
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
