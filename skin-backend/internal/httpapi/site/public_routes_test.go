package site_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/testutil"
)

func TestPublicRoutesCarouselListsOnlyImagesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := site.New(cfg, db, service.Site{DB: db, Cfg: cfg}, nil)
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
