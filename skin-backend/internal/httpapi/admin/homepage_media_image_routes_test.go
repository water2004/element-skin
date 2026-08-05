package admin_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestHomepageMediaImageUploadPatchReorderDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	if err := cache.SetPublicHomepageMedia(context.Background(), []model.HomepageMedia{{ID: "stale"}}, 0); err != nil {
		t.Fatal(err)
	}
	h := admin.NewWithRedis(cfg, db, cache, nil)

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("image upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "image" || item.DurationMS != 6000 || item.SortOrder != 0 || !item.Enabled || item.StoragePath != item.ID+".png" {
		t.Fatalf("image upload item mismatch: %#v", item)
	}
	if item.OverlayOpacityLight != 0.45 || item.OverlayOpacityDark != 0.45 {
		t.Fatalf("image default overlay opacity mismatch: %#v", item)
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.StoragePath)); err != nil {
		t.Fatalf("uploaded image should exist: %v", err)
	}
	if _, err := cache.GetPublicHomepageMedia(context.Background()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("image upload must invalidate public homepage media cache, got %v", err)
	}

	body := strings.NewReader(`{"title":"Hero","enabled":false,"duration_ms":7000,"overlay_opacity_light":0.38,"overlay_opacity_dark":0.62}`)
	req := httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/"+item.ID, body)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch image status=%d body=%q", rec.Code, rec.Body.String())
	}
	patched := decodeMedia(t, rec.Body.Bytes())
	if patched.Title != "Hero" || patched.Enabled || patched.DurationMS != 7000 {
		t.Fatalf("patched image mismatch: %#v", patched)
	}
	if patched.OverlayOpacityLight != 0.38 || patched.OverlayOpacityDark != 0.62 {
		t.Fatalf("patched image overlay opacity mismatch: %#v", patched)
	}

	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "second.png", pngBytes(t, 32, 32)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("second upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	second := decodeMedia(t, rec.Body.Bytes())
	reorderBody := strings.NewReader(`{"ids":["` + second.ID + `","` + item.ID + `"]}`)
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", reorderBody)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("reorder status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[0].SortOrder != 0 || items[1].ID != item.ID || items[1].SortOrder != 1 {
		t.Fatalf("reorder did not persist exact order: %#v", items)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/"+item.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.StoragePath)); !os.IsNotExist(err) {
		t.Fatalf("deleted image file should be gone, stat err=%v", err)
	}
}

func TestHomepageMediaUploadValidatesWebPAndRejectsMalformedMultipartExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/image", strings.NewReader("not multipart"))
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("image malformed multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/panorama", strings.NewReader("not multipart"))
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("panorama malformed multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = multipartUploadRequest(t, "/v2/admin/homepage-media/image", "not_file", "slide.png", pngBytes(t, 8, 8))
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"file is required\"}\n" {
		t.Fatalf("image missing file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "not_file", "panorama.zip", standardPanoramaZip(t))
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"file is required\"}\n" {
		t.Fatalf("panorama missing file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	webp := webpBytes(t)
	req = multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.webp", webp)
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("webp upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "image" || !strings.HasSuffix(item.StoragePath, ".webp") || item.Title != "slide.webp" {
		t.Fatalf("webp homepage media mismatch: %#v", item)
	}
	got, err := os.ReadFile(filepath.Join(cfg.CarouselDir, item.StoragePath))
	if err != nil || !bytes.Equal(got, webp) {
		t.Fatalf("webp upload should persist exact validated bytes: got=%q err=%v", got, err)
	}
}
