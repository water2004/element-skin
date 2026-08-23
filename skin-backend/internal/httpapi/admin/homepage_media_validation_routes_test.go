package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestHomepageMediaMutationsRejectInvalidRequestsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	for _, tc := range []struct {
		name string
		run  func(*httptest.ResponseRecorder)
	}{
		{name: "upload image", run: func(rec *httptest.ResponseRecorder) {
			original := multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8))
			req := httptest.NewRequest(original.Method, original.URL.String(), original.Body)
			req.Header.Set("Content-Type", original.Header.Get("Content-Type"))
			h.UploadHomepageImage(rec, req)
		}},
		{name: "upload panorama", run: func(rec *httptest.ResponseRecorder) {
			original := multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t))
			req := httptest.NewRequest(original.Method, original.URL.String(), original.Body)
			req.Header.Set("Content-Type", original.Header.Get("Content-Type"))
			h.UploadHomepagePanorama(rec, req)
		}},
		{name: "patch", run: func(rec *httptest.ResponseRecorder) {
			h.PatchHomepageMedia(rec, httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{}`)))
		}},
		{name: "reorder", run: func(rec *httptest.ResponseRecorder) {
			h.ReorderHomepageMedia(rec, httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", strings.NewReader(`{"ids":[]}`)))
		}},
		{name: "delete", run: func(rec *httptest.ResponseRecorder) {
			h.DeleteHomepageMedia(rec, httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/missing", nil))
		}},
	} {
		t.Run("permission denied "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.run(rec)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("%s permission mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.gif", []byte("gif")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"upload_file\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n" {
		t.Fatalf("unsupported image mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", []byte("not png")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_image\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid image mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := multipartUploadRequestWithFields(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8), map[string]string{"overlay_opacity_light": "bad"})
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_light_opacity\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("image form opacity parse mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = multipartUploadRequestWithFields(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8), map[string]string{"duration_ms": "999"})
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_duration\",\"operation\":\"validate\",\"reason\":\"out_of_range\"}}\n" {
		t.Fatalf("image duration range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{bad`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("patch invalid json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{"duration_ms":999}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_duration\",\"operation\":\"validate\",\"reason\":\"out_of_range\"}}\n" {
		t.Fatalf("patch duration mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{"overlay_opacity_dark":0.91}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_setting\",\"operation\":\"validate\",\"reason\":\"out_of_range\",\"params\":{\"field\":\"overlay_opacity_dark\"}}}\n" {
		t.Fatalf("patch opacity mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{"title":"x"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("patch missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "request.decode.invalid", body: `{bad`},
		{name: "empty ids", body: `{"ids":[]}`},
		{name: "duplicate ids", body: `{"ids":["a","a"]}`},
		{name: "blank id", body: `{"ids":[""]}`},
	} {
		t.Run("reorder "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", strings.NewReader(tc.body))
			req = withAdminActor(req, "admin-test-user")
			h.ReorderHomepageMedia(rec, req)
			wantBody := "{\"error\":{\"object\":\"id_list\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"
			if tc.name == "request.decode.invalid" {
				wantBody = "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n"
			}
			if tc.name == "empty ids" {
				wantBody = "{\"error\":{\"object\":\"id_list\",\"operation\":\"validate\",\"reason\":\"required\"}}\n"
			}
			if rec.Code != http.StatusBadRequest || rec.Body.String() != wantBody {
				t.Fatalf("reorder %s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", strings.NewReader(`{"ids":["missing"]}`))
	req = withAdminActor(req, "admin-test-user")
	h.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("reorder missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/missing", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("delete missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
