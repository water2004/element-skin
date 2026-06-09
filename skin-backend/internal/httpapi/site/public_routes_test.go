package site_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestPublicRoutesCarouselListsOnlyImagesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	if err := os.WriteFile(cfg.CarouselDir+"\\hero.webp", []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CarouselDir+"\\notes.txt", []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public/carousel", nil)
	rec := httptest.NewRecorder()
	h.PublicCarousel(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "[\"hero.webp\"]\n" {
		t.Fatalf("public carousel should list only images exactly: status=%d body=%q", rec.Code, rec.Body.String())
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
	h.PublicCarousel(rec, httptest.NewRequest(http.MethodGet, "/public/carousel", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public carousel redis error should fail, got %d body=%q", rec.Code, rec.Body.String())
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

func TestPublicRoutesUseRedisCachedSettingsAndCarouselExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := redisstore.NewMemoryStore()
	if err := cache.SetPublicSettings(context.Background(), map[string]any{
		"site_name":          "Cached Site",
		"allow_register":     false,
		"mojang_status_urls": map[string]any{"session": "cached-session"},
		"cached_only_marker": true,
	}, time.Duration(cfg.PublicCacheTTL)*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetPublicCarousel(context.Background(), []string{"cached.webp"}, time.Duration(cfg.PublicCacheTTL)*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CarouselDir+"\\disk.webp", []byte("img"), 0o644); err != nil {
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
	h.PublicCarousel(rec, httptest.NewRequest(http.MethodGet, "/public/carousel", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "[\"cached.webp\"]\n" {
		t.Fatalf("public carousel should return cached payload instead of disk scan: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesCarouselMissingDirectoryCachesEmptyList(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir() + "\\missing-carousel"
	cache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)

	rec := httptest.NewRecorder()
	h.PublicCarousel(rec, httptest.NewRequest(http.MethodGet, "/public/carousel", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("missing carousel directory should return empty list: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetPublicCarousel(context.Background())
	if err != nil || len(cached) != 0 {
		t.Fatalf("missing carousel directory result should be cached as empty list: %#v err=%v", cached, err)
	}
}

func TestPublicRoutesFailWhenRedisCannotStorePublicCaches(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	settingsCache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(cfg, db, &writeFailRedis{Store: settingsCache}, sitesvc.Site{DB: db, Cfg: cfg, Redis: settingsCache}, nil)

	rec := httptest.NewRecorder()
	h.PublicSettings(rec, httptest.NewRequest(http.MethodGet, "/public/settings", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public settings should fail when redis cache write fails: status=%d body=%q", rec.Code, rec.Body.String())
	}

	carouselCache := redisstore.NewMemoryStore()
	h = site.NewWithRedis(cfg, db, &writeFailRedis{Store: carouselCache}, sitesvc.Site{DB: db, Cfg: cfg, Redis: carouselCache}, nil)
	rec = httptest.NewRecorder()
	h.PublicCarousel(rec, httptest.NewRequest(http.MethodGet, "/public/carousel", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public carousel should fail when redis cache write fails: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type writeFailRedis struct {
	redisstore.Store
}

func (r *writeFailRedis) SetPublicSettings(context.Context, map[string]any, time.Duration) error {
	return errors.New("redis write failed")
}

func (r *writeFailRedis) SetPublicCarousel(context.Context, []string, time.Duration) error {
	return errors.New("redis write failed")
}
