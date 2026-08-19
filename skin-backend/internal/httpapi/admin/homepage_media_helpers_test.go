package admin

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/util"
)

func TestHomepageMediaFormHelpersParseDefaultsAndExactErrors(t *testing.T) {
	values, err := homepagesvc.ParseMediaValues(map[string]string{}, "image")
	if err != nil {
		t.Fatal(err)
	}
	if values.OverlayOpacityLight != 0.45 || values.OverlayOpacityDark != 0.45 ||
		values.StartYaw != 0 || values.StartPitch != 0 || values.YawSpeedDPS != 4 || values.PitchSpeedDPS != 0 {
		t.Fatalf("default image values mismatch: %#v", values)
	}
	_, err = homepagesvc.ParseMediaValues(map[string]string{"overlay_opacity_light": "bad"}, "image")
	assertHTTPError(t, err, 400, "homepage_light_opacity.validate.invalid")

	_, err = homepagesvc.ParseMediaValues(map[string]string{"overlay_opacity_light": "0.91"}, "image")
	assertHTTPError(t, err, 400, "homepage_setting.validate.out_of_range")

	_, err = homepagesvc.ParseMediaValues(map[string]string{"start_yaw": "abc"}, "panorama")
	assertHTTPError(t, err, 400, "homepage_start_yaw.validate.invalid")

	_, err = homepagesvc.ParseMediaValues(map[string]string{"start_yaw": "361"}, "panorama")
	assertHTTPError(t, err, 400, "homepage_start_yaw.validate.out_of_range")

	values, err = homepagesvc.ParseMediaValues(map[string]string{"duration_ms": "not-number"}, "image")
	if err != nil || values.DurationMS != 0 {
		t.Fatalf("invalid duration should fall back to service default marker: values=%#v err=%v", values, err)
	}
}

func TestReadPanoramaZipRejectsExactInvalidShapes(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		detail string
	}{
		{name: "not zip", data: []byte("not a zip"), detail: "panorama_archive.decode.invalid"},
		{name: "nested", data: panoramaZipWithFiles(t, map[string][]byte{"nested/panorama_0.png": pngBytesForAdminHelper(t, 4, 4)}), detail: "panorama_archive.validate.invalid"},
		{name: "extra", data: panoramaZipWithFiles(t, map[string][]byte{"panorama_0.png": pngBytesForAdminHelper(t, 4, 4), "other.png": pngBytesForAdminHelper(t, 4, 4)}), detail: "panorama_archive.validate.invalid"},
		{name: "invalid face", data: panoramaZipWithFiles(t, panoramaFaces(t, map[string][]byte{"panorama_3.png": []byte("bad png")})), detail: "panorama_face.validate.invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			faces, err := homepagesvc.ReadPanoramaZip(tc.data)
			if faces != nil {
				t.Fatalf("invalid panorama should not return faces: %#v", faces)
			}
			assertHTTPError(t, err, 400, tc.detail)
		})
	}

	faces, err := homepagesvc.ReadPanoramaZip(panoramaZipWithFiles(t, panoramaFaces(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != 6 || len(faces["panorama_0.png"]) == 0 || len(faces["panorama_5.png"]) == 0 {
		t.Fatalf("valid panorama faces mismatch: keys=%v", faces)
	}
}

func TestValidateHomepageMediaValuesExactBoundaries(t *testing.T) {
	zero := 0.0
	if err := homepagesvc.ValidateOpacity("overlay_opacity_light", nil); err != nil {
		t.Fatal(err)
	}
	if err := homepagesvc.ValidateOpacity("overlay_opacity_light", &zero); err != nil {
		t.Fatal(err)
	}
	negative := -0.01
	assertHTTPError(t, homepagesvc.ValidateOpacity("overlay_opacity_light", &negative), 400, "homepage_setting.validate.out_of_range")

	startPitch := -90.0
	assertHTTPError(t, homepagesvc.ValidatePanoramaValues(nil, &startPitch, nil, nil), 400, "homepage_start_pitch.validate.out_of_range")
	yawSpeed := -91.0
	assertHTTPError(t, homepagesvc.ValidatePanoramaValues(nil, nil, &yawSpeed, nil), 400, "homepage_yaw_speed.validate.out_of_range")
	pitchSpeed := 91.0
	assertHTTPError(t, homepagesvc.ValidatePanoramaValues(nil, nil, nil, &pitchSpeed), 400, "homepage_pitch_speed.validate.out_of_range")
}

func panoramaFaces(t *testing.T, overrides map[string][]byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for i := 0; i < 6; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		out[name] = pngBytesForAdminHelper(t, 4, 4)
	}
	for name, data := range overrides {
		out[name] = data
	}
	return out
}

func panoramaZipWithFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngBytesForAdminHelper(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 210, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Error() != detail {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}
