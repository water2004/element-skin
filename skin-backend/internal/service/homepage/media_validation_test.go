package homepage_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
)

func TestHomepageServiceRejectsPermissionsAndInvalidInputsExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: t.TempDir()}
	actor := homepageActor("homepage_media.create.any", "homepage_media.update.any")

	if _, err := svc.List(ctx, permission.Actor{}); !homepageHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("List without permission mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, permission.Actor{}, newMultipartSource("file", "hero.png", tinyPNGBytes(t), nil)); !homepageHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UploadImage without permission mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "notes.txt", []byte("not image"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "upload_file.validate.unsupported") {
		t.Fatalf("unsupported image extension mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "broken.png", []byte("not image"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "homepage_image.decode.invalid") {
		t.Fatalf("invalid image mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "fake.webp", []byte("not webp"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "homepage_image.decode.invalid") {
		t.Fatalf("invalid webp mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "mismatch.webp", tinyPNGBytes(t), nil)); !homepageHTTPError(err, http.StatusBadRequest, "homepage_image.decode.invalid") {
		t.Fatalf("extension and image format mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newFieldsOnlyMultipartSource(map[string]string{"title": "missing"})); !homepageHTTPError(err, http.StatusBadRequest, "upload_file.validate.required") {
		t.Fatalf("missing image file mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "huge.webp", bytes.Repeat([]byte("x"), homepagesvc.MaxImageBytes+1), nil)); !homepageHTTPError(err, http.StatusBadRequest, "upload_file.validate.too_large") {
		t.Fatalf("oversized image mismatch: %#v", err)
	}
	if _, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "sky.txt", []byte("zip"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "upload_file.validate.unsupported") {
		t.Fatalf("unsupported panorama extension mismatch: %#v", err)
	}
	if _, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "sky.zip", []byte("zip"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "panorama_archive.decode.invalid") {
		t.Fatalf("invalid panorama zip mismatch: %#v", err)
	}
	if _, err := svc.Patch(ctx, actor, "missing", homepagesvc.PatchInput{DurationMS: intPtr(999)}); !homepageHTTPError(err, http.StatusBadRequest, "homepage_duration.validate.out_of_range") {
		t.Fatalf("invalid patch duration mismatch: %#v", err)
	}
	if err := svc.Reorder(ctx, actor, nil); !homepageHTTPError(err, http.StatusBadRequest, "id_list.validate.required") {
		t.Fatalf("empty reorder mismatch: %#v", err)
	}
	if err := svc.Reorder(ctx, actor, []string{"same", "same"}); !homepageHTTPError(err, http.StatusBadRequest, "id_list.validate.invalid") {
		t.Fatalf("duplicate reorder mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, permission.Actor{}, "missing"); !homepageHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("Delete without permission mismatch: %#v", err)
	}
}

func TestHomepageMediaParsingAndPanoramaZipValidationExactErrors(t *testing.T) {
	values, err := homepagesvc.ParseMediaValues(map[string]string{
		"overlay_opacity_light": "0.3",
		"overlay_opacity_dark":  "0.7",
		"start_yaw":             "45",
		"start_pitch":           "-20",
		"yaw_speed_dps":         "-5",
		"pitch_speed_dps":       "6",
		"duration_ms":           "9000",
	}, "panorama")
	if err != nil {
		t.Fatal(err)
	}
	if values.OverlayOpacityLight != 0.3 || values.OverlayOpacityDark != 0.7 || values.StartYaw != 45 ||
		values.StartPitch != -20 || values.YawSpeedDPS != -5 || values.PitchSpeedDPS != 6 || values.DurationMS != 9000 {
		t.Fatalf("parsed media values mismatch: %#v", values)
	}

	for _, tc := range []struct {
		name   string
		fields map[string]string
		typ    string
		detail string
	}{
		{"bad light opacity", map[string]string{"overlay_opacity_light": "bad"}, "image", "homepage_light_opacity.validate.invalid"},
		{"dark opacity range", map[string]string{"overlay_opacity_dark": "1"}, "image", "homepage_setting.validate.out_of_range"},
		{"bad start yaw", map[string]string{"start_yaw": "bad"}, "panorama", "homepage_start_yaw.validate.invalid"},
		{"start pitch range", map[string]string{"start_pitch": "-90"}, "panorama", "homepage_start_pitch.validate.out_of_range"},
		{"yaw speed range", map[string]string{"yaw_speed_dps": "91"}, "panorama", "homepage_yaw_speed.validate.out_of_range"},
		{"pitch speed range", map[string]string{"pitch_speed_dps": "-91"}, "panorama", "homepage_pitch_speed.validate.out_of_range"},
		{"bad duration fallback", map[string]string{"duration_ms": "not-number"}, "image", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := homepagesvc.ParseMediaValues(tc.fields, tc.typ)
			if tc.detail == "" {
				if err != nil || got.DurationMS != 0 {
					t.Fatalf("ParseMediaValues fallback mismatch: values=%#v err=%#v", got, err)
				}
				return
			}
			if !homepageHTTPError(err, http.StatusBadRequest, tc.detail) || got != (homepagesvc.MediaValues{}) {
				t.Fatalf("ParseMediaValues error mismatch: values=%#v err=%#v", got, err)
			}
		})
	}

	if faces, err := homepagesvc.ReadPanoramaZip(validPanoramaZip(t)); err != nil || len(faces) != 6 || !bytes.Equal(faces["panorama_0.png"], tinyPNGBytes(t)) {
		t.Fatalf("valid panorama zip mismatch: faces=%#v err=%v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"nested/panorama_0.png": tinyPNGBytes(t)})); !homepageHTTPError(err, http.StatusBadRequest, "panorama_archive.validate.invalid") || faces != nil {
		t.Fatalf("nested panorama zip mismatch: faces=%#v err=%#v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"panorama_6.png": tinyPNGBytes(t)})); !homepageHTTPError(err, http.StatusBadRequest, "panorama_archive.validate.invalid") || faces != nil {
		t.Fatalf("extra panorama zip mismatch: faces=%#v err=%#v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"panorama_0.png": []byte("not image")})); !homepageHTTPError(err, http.StatusBadRequest, "panorama_face.validate.invalid") || faces != nil {
		t.Fatalf("invalid panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
	missingZero := map[string][]byte{}
	for i := 1; i < 6; i++ {
		missingZero["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, missingZero)); !homepageHTTPError(err, http.StatusBadRequest, "panorama_face.validate.required") || faces != nil {
		t.Fatalf("missing panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
	oversized := map[string][]byte{}
	for i := 0; i < 6; i++ {
		oversized["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	oversized["panorama_3.png"] = bytes.Repeat([]byte("x"), homepagesvc.MaxImageBytes+1)
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, oversized)); !homepageHTTPError(err, http.StatusBadRequest, "panorama_face.validate.too_large") || faces != nil {
		t.Fatalf("oversized panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
}
