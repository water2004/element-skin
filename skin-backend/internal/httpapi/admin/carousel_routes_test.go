package admin_test

import (
	"bytes"
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
	"element-skin/backend/internal/testutil"
)

func TestCarouselRoutesRejectUnsupportedUploadFormat(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "slide.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not an image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	h := admin.New(testutil.TestConfig(), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/carousel", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Unsupported file format") {
		t.Fatalf("unsupported carousel upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCarouselRoutesUploadAndDeleteExactFileState(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.New(cfg, nil, nil)
	req := carouselUploadRequest(t, "slide.png", carouselPNG(t))
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"filename":"`) || !strings.Contains(rec.Body.String(), `.png"`) {
		t.Fatalf("carousel upload response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	filename := responseStringField(t, rec.Body.String(), "filename")
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, filename)); err != nil {
		t.Fatalf("uploaded carousel file should exist: %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/carousel/"+filename, nil)
	req.SetPathValue("filename", filename)
	rec = httptest.NewRecorder()
	h.DeleteCarousel(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("carousel delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, filename)); !os.IsNotExist(err) {
		t.Fatalf("carousel file should be deleted, stat err=%v", err)
	}
}

func carouselUploadRequest(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/carousel", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func carouselPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.SetRGBA(x, y, color.RGBA{R: 20, G: 40, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func responseStringField(t *testing.T, body, field string) string {
	t.Helper()
	marker := `"` + field + `":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing field %s in %q", field, body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated field %s in %q", field, body)
	}
	return body[start : start+end]
}
