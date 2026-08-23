package admin_test

import (
	"bytes"
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

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusCreated {
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

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/"+item.ID, strings.NewReader(`{"start_yaw":30,"start_pitch":-10,"yaw_speed_dps":-3.5,"pitch_speed_dps":2.25}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch panorama status=%d body=%q", rec.Code, rec.Body.String())
	}
	patched := decodeMedia(t, rec.Body.Bytes())
	if patched.StartYaw != 30 || patched.StartPitch != -10 || patched.YawSpeedDPS != -3.5 || patched.PitchSpeedDPS != 2.25 {
		t.Fatalf("patched panorama fields mismatch: %#v", patched)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/"+item.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete panorama status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.ID)); !os.IsNotExist(err) {
		t.Fatalf("deleted panorama directory should be gone, stat err=%v", err)
	}
}

func TestHomepageMediaRejectsInvalidPanoramaInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.NewWithRedis(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)

	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "bad.txt", []byte("x")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"upload_file\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n" {
		t.Fatalf("bad extension mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "bad.zip", invalidPanoramaZip(t)))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"panorama_face\",\"operation\":\"validate\",\"reason\":\"required\",\"params\":{\"face\":\"panorama_5.png\"}}}\n" {
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
	req := httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_start_pitch\",\"operation\":\"validate\",\"reason\":\"out_of_range\"}}\n" {
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
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_yaw_speed\",\"operation\":\"validate\",\"reason\":\"out_of_range\"}}\n" {
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
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_setting\",\"operation\":\"validate\",\"reason\":\"out_of_range\",\"params\":{\"field\":\"overlay_opacity_dark\"}}}\n" {
		t.Fatalf("overlay range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		name   string
		fields map[string]string
		body   string
	}{
		{name: "overlay opacity dark parse", fields: map[string]string{"overlay_opacity_dark": "bad"}, body: "{\"error\":{\"object\":\"homepage_dark_opacity\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "start yaw parse", fields: map[string]string{"start_yaw": "bad"}, body: "{\"error\":{\"object\":\"homepage_start_yaw\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "start pitch parse", fields: map[string]string{"start_pitch": "bad"}, body: "{\"error\":{\"object\":\"homepage_start_pitch\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "yaw speed parse", fields: map[string]string{"yaw_speed_dps": "bad"}, body: "{\"error\":{\"object\":\"homepage_yaw_speed\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "pitch speed parse", fields: map[string]string{"pitch_speed_dps": "bad"}, body: "{\"error\":{\"object\":\"homepage_pitch_speed\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := multipartUploadRequestWithFields(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t), tc.fields)
			h.UploadHomepagePanorama(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != tc.body {
				t.Fatalf("%s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	for _, tc := range []struct {
		name string
		data []byte
		body string
	}{
		{name: "not zip", data: []byte("not a zip archive"), body: "{\"error\":{\"object\":\"panorama_archive\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n"},
		{name: "nested file", data: panoramaZipWithEntries(t, map[string][]byte{"nested/panorama_0.png": pngBytes(t, 8, 8)}), body: "{\"error\":{\"object\":\"panorama_archive\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "unexpected file", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": pngBytes(t, 8, 8), "extra.png": pngBytes(t, 8, 8)}), body: "{\"error\":{\"object\":\"panorama_archive\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
		{name: "oversized face", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": bytes.Repeat([]byte{'x'}, maxHomepageFaceBytesForTest()+1)}), body: "{\"error\":{\"object\":\"panorama_face\",\"operation\":\"validate\",\"reason\":\"too_large\"}}\n"},
		{name: "invalid face image", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": []byte("not png")}), body: "{\"error\":{\"object\":\"panorama_face\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", tc.data))
			if rec.Code != http.StatusBadRequest || rec.Body.String() != tc.body {
				t.Fatalf("%s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h = admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("panorama setup status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/"+item.ID, strings.NewReader(`{"start_yaw":361}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"homepage_start_yaw\",\"operation\":\"validate\",\"reason\":\"out_of_range\"}}\n" {
		t.Fatalf("patch panorama yaw range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
