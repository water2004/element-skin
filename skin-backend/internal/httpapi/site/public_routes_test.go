package site_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestPublicRoutesHomepageMediaListsEnabledItemsFromDBExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	now := database.NowMS()
	if err := db.HomepageMedia.Create(context.Background(), model.HomepageMedia{
		ID: "disabled", Type: "image", Title: "Disabled", StoragePath: "disabled.webp", Config: map[string]any{}, SortOrder: 0, Enabled: false, DurationMS: 6000, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.HomepageMedia.Create(context.Background(), model.HomepageMedia{
		ID: "hero", Type: "image", Title: "Hero", StoragePath: "hero.webp", Config: map[string]any{}, SortOrder: 1, Enabled: true, DurationMS: 6000, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public/homepage-media", nil)
	rec := httptest.NewRecorder()
	h.PublicHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"hero"`) || strings.Contains(rec.Body.String(), `"id":"disabled"`) {
		t.Fatalf("public homepage media should list only enabled DB rows exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesRedisErrorDoesNotFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)

	rec := httptest.NewRecorder()
	h.PublicSettings(rec, httptest.NewRequest(http.MethodGet, "/public/settings", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public settings redis error should fail, got %d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.PublicHomepageMedia(rec, httptest.NewRequest(http.MethodGet, "/public/homepage-media", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public homepage media redis error should fail, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesSettingsAndLibraryExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "public-routes@test.com", "Password123", "PublicRoutes", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "public_route_hash", "skin", "Public Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/public/settings", nil)
	rec := httptest.NewRecorder()
	h.PublicSettings(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) || !strings.Contains(rec.Body.String(), `"enable_skin_library"`) || !strings.Contains(rec.Body.String(), `"easter_eggs"`) {
		t.Fatalf("public settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/public/skin-library?texture_type=skin&q=Public%20Route", nil)
	rec = httptest.NewRecorder()
	h.PublicLibrary(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"public_route_hash"`) || !strings.Contains(rec.Body.String(), `"name":"Public Route Texture"`) {
		t.Fatalf("public library response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesUseRedisCachedSettingsAndHomepageMediaExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	if err := cache.SetPublicSettings(context.Background(), map[string]any{
		"site_name":          "Cached Site",
		"allow_register":     false,
		"mojang_status_urls": map[string]any{"session": "cached-session"},
		"cached_only_marker": true,
	}, time.Duration(cfg.PublicCacheTTL)*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetPublicHomepageMedia(context.Background(), []model.HomepageMedia{{ID: "cached", Type: "image", StoragePath: "cached.webp", Enabled: true}}, time.Duration(cfg.PublicCacheTTL)*time.Second); err != nil {
		t.Fatal(err)
	}
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)

	rec := httptest.NewRecorder()
	h.PublicSettings(rec, httptest.NewRequest(http.MethodGet, "/public/settings", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name":"Cached Site"`) ||
		!strings.Contains(rec.Body.String(), `"cached_only_marker":true`) {
		t.Fatalf("public settings should return cached payload exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.PublicHomepageMedia(rec, httptest.NewRequest(http.MethodGet, "/public/homepage-media", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"cached"`) || !strings.Contains(rec.Body.String(), `"storage_path":"cached.webp"`) {
		t.Fatalf("public homepage media should return cached payload instead of DB: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesHomepageMediaEmptyDBCachesEmptyList(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)

	rec := httptest.NewRecorder()
	h.PublicHomepageMedia(rec, httptest.NewRequest(http.MethodGet, "/public/homepage-media", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("empty homepage media table should return empty list: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetPublicHomepageMedia(context.Background())
	if err != nil || len(cached) != 0 {
		t.Fatalf("empty homepage media result should be cached as empty list: %#v err=%v", cached, err)
	}
}

func TestPublicRoutesFailWhenRedisCannotStorePublicCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	settingsCache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, &writeFailRedis{Store: settingsCache}, sitesvc.Site{DB: db, Cfg: cfg, Redis: settingsCache}, nil)

	rec := httptest.NewRecorder()
	h.PublicSettings(rec, httptest.NewRequest(http.MethodGet, "/public/settings", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public settings should fail when redis cache write fails: status=%d body=%q", rec.Code, rec.Body.String())
	}

	homepageCache := redisstore.NewMemoryStore()
	h = site.NewWithRedis(cfg, db, &writeFailRedis{Store: homepageCache}, sitesvc.Site{DB: db, Cfg: cfg, Redis: homepageCache}, nil)
	rec = httptest.NewRecorder()
	h.PublicHomepageMedia(rec, httptest.NewRequest(http.MethodGet, "/public/homepage-media", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public homepage media should fail when redis cache write fails: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type writeFailRedis struct {
	redisstore.Store
}

func (r *writeFailRedis) SetPublicSettings(context.Context, map[string]any, time.Duration) error {
	return errors.New("redis write failed")
}

func (r *writeFailRedis) SetPublicHomepageMedia(context.Context, []model.HomepageMedia, time.Duration) error {
	return errors.New("redis write failed")
}

func TestPublicFallbackStatusReturnsEndpointHistoryAndCurrent(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	ctx := context.Background()

	svc := settings.Settings{DB: db, Redis: cache}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{
			map[string]any{
				"priority":     1,
				"session_url":  "https://session.example",
				"account_url":  "https://account.example",
				"services_url": "https://services.example",
				"note":         "primary",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(endpoints) != 1 {
		t.Fatalf("seed endpoints: %v %#v", err, endpoints)
	}
	endpointID, _ := endpoints[0]["id"].(int)

	now := time.Now()
	older := redisstore.ProbeSample{EndpointID: endpointID, Note: "primary", CheckedAt: now.Add(-30 * time.Minute).UnixMilli(), Session: "down", Account: "up", Services: "up"}
	newest := redisstore.ProbeSample{EndpointID: endpointID, Note: "primary", CheckedAt: now.UnixMilli(), Session: "up", Account: "up", Services: "up"}
	if err := cache.AppendProbeSamples(ctx, []redisstore.ProbeSample{older, newest}, time.Hour); err != nil {
		t.Fatal(err)
	}

	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)
	rec := httptest.NewRecorder()
	h.PublicFallbackStatus(rec, httptest.NewRequest(http.MethodGet, "/public/fallback-status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Endpoints []struct {
			ID          int    `json:"id"`
			Note        string `json:"note"`
			SessionURL  string `json:"session_url"`
			AccountURL  string `json:"account_url"`
			ServicesURL string `json:"services_url"`
			Latest      *struct {
				Session string `json:"session"`
				Account string `json:"account"`
			} `json:"latest"`
			History []struct {
				Session string `json:"session"`
				Account string `json:"account"`
			} `json:"history"`
		} `json:"endpoints"`
		RetentionMS int64 `json:"retention_ms"`
		GeneratedAt int64 `json:"generated_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(body.Endpoints) != 1 {
		t.Fatalf("expected one endpoint in payload, got %d: %s", len(body.Endpoints), rec.Body.String())
	}
	ep := body.Endpoints[0]
	if ep.Note != "primary" || ep.SessionURL != "https://session.example" || ep.AccountURL != "https://account.example" || ep.ServicesURL != "https://services.example" {
		t.Fatalf("endpoint metadata mismatch: %#v", ep)
	}
	if len(ep.History) != 2 || ep.History[0].Session != "down" || ep.History[1].Session != "up" {
		t.Fatalf("history should be ordered oldest→newest: %#v", ep.History)
	}
	if ep.Latest == nil || ep.Latest.Session != "up" || ep.Latest.Account != "up" {
		t.Fatalf("latest should reflect the newest sample: %#v", ep.Latest)
	}
	if body.RetentionMS != int64(redisstore.ProbeHistoryRetention.Milliseconds()) {
		t.Fatalf("retention_ms should match constant: got %d", body.RetentionMS)
	}
}

func TestPublicFallbackStatusReturnsEmptyWhenNoConfig(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)
	rec := httptest.NewRecorder()
	h.PublicFallbackStatus(rec, httptest.NewRequest(http.MethodGet, "/public/fallback-status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"endpoints":[]`) {
		t.Fatalf("expected empty endpoints array: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicFallbackStatusFailsWhenRedisErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)
	rec := httptest.NewRecorder()
	h.PublicFallbackStatus(rec, httptest.NewRequest(http.MethodGet, "/public/fallback-status", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis error should yield 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}
