package admin_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
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

func TestListHomepageMedia(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	t.Run("empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/homepage-media", nil)
		req = withAdminActor(req, "admin-test-user")
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("empty list status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "[]\n" {
			t.Fatalf("empty list must be [], got %q", rec.Body.String())
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/homepage-media", nil)
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("permission denied status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
			t.Fatalf("permission denied body mismatch: %q", rec.Body.String())
		}
	})
}

func TestHomepageMediaUploadFailsWhenRedisInvalidateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, &homepageInvalidateFailRedis{Store: testutil.NewMemoryRedis()}, nil)

	req := multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64))
	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis fail upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("redis fail upload body mismatch: %q", rec.Body.String())
	}
	// Verify DB record was cleaned up after Redis failure.
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items after Redis invalidate failure, got %d", len(items))
	}
	// Verify file was cleaned up.
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files after Redis invalidate failure, got %d", len(entries))
	}
}

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
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64)))
	if rec.Code != http.StatusOK {
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
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/"+item.ID, body)
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
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "second.png", pngBytes(t, 32, 32)))
	if rec.Code != http.StatusOK {
		t.Fatalf("second upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	second := decodeMedia(t, rec.Body.Bytes())
	reorderBody := strings.NewReader(`{"ids":["` + second.ID + `","` + item.ID + `"]}`)
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", reorderBody)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reorder status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[0].SortOrder != 0 || items[1].ID != item.ID || items[1].SortOrder != 1 {
		t.Fatalf("reorder did not persist exact order: %#v", items)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/"+item.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.StoragePath)); !os.IsNotExist(err) {
		t.Fatalf("deleted image file should be gone, stat err=%v", err)
	}
}

func TestHomepageMediaPanoramaUploadUsesGeneratedStandardZipAndYawPitchFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"overlay_opacity_light": "0.25", "overlay_opacity_dark": "0.55", "start_yaw": "-45", "start_pitch": "5", "yaw_speed_dps": "6", "pitch_speed_dps": "-1.5", "duration_ms": "11000",
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("panorama upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "panorama" || item.DurationMS != 11000 || item.StoragePath != item.ID {
		t.Fatalf("panorama item mismatch: %#v", item)
	}
	if item.OverlayOpacityLight != 0.25 || item.OverlayOpacityDark != 0.55 || item.StartYaw != -45 || item.StartPitch != 5 || item.YawSpeedDPS != 6 || item.PitchSpeedDPS != -1.5 {
		t.Fatalf("panorama fields mismatch: %#v", item)
	}
	for _, name := range []string{
		"panorama_0.png",
		"panorama_1.png",
		"panorama_2.png",
		"panorama_3.png",
		"panorama_4.png",
		"panorama_5.png",
	} {
		if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.ID, name)); err != nil {
			t.Fatalf("panorama face %s should exist: %v", name, err)
		}
	}
}

func TestHomepageMediaRejectsInvalidPanoramaInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.NewWithRedis(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)

	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "bad.txt", []byte("x")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Unsupported file format\"}\n" {
		t.Fatalf("bad extension mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "bad.zip", invalidPanoramaZip(t)))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"missing panorama_5.png\"}\n" {
		t.Fatalf("missing face mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("start_pitch", "91"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"start_pitch out of range\"}\n" {
		t.Fatalf("pitch range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("yaw_speed_dps", "91"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"yaw_speed_dps out of range\"}\n" {
		t.Fatalf("yaw speed range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("overlay_opacity_dark", "1"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"overlay_opacity_dark out of range\"}\n" {
		t.Fatalf("overlay range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type homepageInvalidateFailRedis struct {
	redisstore.Store
}

func (r *homepageInvalidateFailRedis) InvalidatePublicHomepageMedia(context.Context) error {
	return errors.New("redis invalidate failed")
}

func multipartUploadRequest(t *testing.T, path, field, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.SetRGBA(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func standardPanoramaZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 6; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(pngBytes(t, 16, 16)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func invalidPanoramaZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 5; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(pngBytes(t, 16, 16)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeMedia(t *testing.T, raw []byte) model.HomepageMedia {
	t.Helper()
	var item model.HomepageMedia
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("decode media %q: %v", raw, err)
	}
	return item
}
