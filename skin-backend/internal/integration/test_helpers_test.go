package integration_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"element-skin/backend/internal/redisstore"
)

var testRequestIP atomic.Uint64

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return doJSONFromIP(t, h, method, path, body, nextTestRemoteAddr(), cookies...)
}

func doJSONFromIP(t *testing.T, h http.Handler, method, path string, body any, remoteAddr string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &b)
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		if c == nil {
			continue
		}
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func doRawJSON(t *testing.T, h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = nextTestRemoteAddr()
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		if c == nil {
			continue
		}
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func parseJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode json %q: %v", rr.Body.String(), err)
	}
	return out
}

func cookieNamed(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func invalidateSettings(t *testing.T, redis redisstore.Store) {
	t.Helper()
	if err := redis.InvalidateSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func pngTexture(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func doMultipart(t *testing.T, h http.Handler, method, path string, fields map[string]string, fileField, fileName string, fileBytes []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		part, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, &b)
	req.RemoteAddr = nextTestRemoteAddr()
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range cookies {
		if c == nil {
			continue
		}
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func nextTestRemoteAddr() string {
	n := testRequestIP.Add(1)
	return "192.0.2." + strconv.Itoa(int(n%250+1)) + ":" + strconv.Itoa(10_000+int(n%50_000))
}
