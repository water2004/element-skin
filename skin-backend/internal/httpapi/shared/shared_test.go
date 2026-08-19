package shared_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"

	"element-skin/backend/internal/httpapi/shared"
)

func TestRequestContextAndValueHelpersPreserveExactValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if shared.CurrentActor(req).Permissions != nil {
		t.Fatal("request without auth context must not contain permissions")
	}

	req = req.WithContext(shared.WithActorPermissions(context.Background(), "user-123", permission.MustDefinitionByCode("permission_protected.manage.any")))
	if got := shared.CurrentActor(req).UserID; got != "user-123" {
		t.Fatalf("context actor user ID=%q, want user-123", got)
	}
	if !shared.CurrentActor(req).Has(permission.MustDefinitionByCode("permission_protected.manage.any")) {
		t.Fatal("explicit protected permission should be preserved")
	}
	if got := shared.AsString("exact"); got != "exact" {
		t.Fatalf("AsString(string)=%q, want exact", got)
	}
	if got := shared.AsString(123); got != "" {
		t.Fatalf("AsString(non-string)=%q, want empty", got)
	}
}

func TestParsePositiveIntFormBoolAndDecodeJSONContracts(t *testing.T) {
	for raw, want := range map[string]int{
		"1":    1,
		" 42 ": 42,
		"0007": 7,
	} {
		got, err := shared.ParsePositiveInt(raw)
		if err != nil || got != want {
			t.Fatalf("ParsePositiveInt(%q)=%d, %v; want %d, nil", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "0", "-1", "1.5", "abc"} {
		if got, err := shared.ParsePositiveInt(raw); err == nil || got != 0 || err.Error() != "invalid positive int" {
			t.Fatalf("ParsePositiveInt(%q)=%d, %v; want 0, invalid positive int", raw, got, err)
		}
	}

	for _, raw := range []string{"true", " TRUE ", "1", "yes", "On"} {
		if !shared.FormBool(raw) {
			t.Fatalf("FormBool(%q)=false, want true", raw)
		}
	}
	for _, raw := range []string{"", "false", "0", "no", "off", "2"} {
		if shared.FormBool(raw) {
			t.Fatalf("FormBool(%q)=true, want false", raw)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	trackedBody := &trackingReadCloser{Reader: bytes.NewBufferString(`{"name":"Alice","count":2}`)}
	req.Body = trackedBody
	var body struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		t.Fatal(err)
	}
	if body.Name != "Alice" || body.Count != 2 {
		t.Fatalf("decoded body=%#v, want exact JSON values", body)
	}
	if !trackedBody.closed {
		t.Fatal("DecodeJSON must close the request body")
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":`))
	if err := shared.DecodeJSON(req, &body); err == nil {
		t.Fatal("DecodeJSON must return malformed JSON errors")
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"first"} {"name":"second"}`))
	if err := shared.DecodeJSON(req, &body); !errors.Is(err, shared.ErrMultipleJSONValues) {
		t.Fatalf("DecodeJSON multiple values err=%v, want ErrMultipleJSONValues", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bytes.Repeat([]byte("x"), shared.MaxJSONBodyBytes+1)))
	if err := shared.DecodeJSON(req, &body); !errors.Is(err, shared.ErrJSONBodyTooLarge) {
		t.Fatalf("DecodeJSON oversized body err=%v, want ErrJSONBodyTooLarge", err)
	}
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestParseImportProfilesValidatesShapeAndTrimsValues(t *testing.T) {
	profiles, err := shared.ParseImportProfiles([]any{
		map[string]any{"profile_id": "  id-one  ", "profile_name": "  NameOne  "},
		map[string]any{"profile_id": "id-two", "profile_name": "NameTwo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || profiles[0]["profile_id"] != "id-one" || profiles[0]["profile_name"] != "NameOne" || profiles[1]["profile_id"] != "id-two" {
		t.Fatalf("unexpected parsed profiles: %#v", profiles)
	}

	for _, raw := range []any{nil, "not-list", []any{}, []any{"not-map"}} {
		if _, err := shared.ParseImportProfiles(raw); err == nil {
			t.Fatalf("shared.ParseImportProfiles(%#v) should reject invalid shape", raw)
		}
	}
}

func TestBearerTokenRequiresBearerSchemeAndNonEmptyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if token, ok := shared.BearerToken(req); ok || token != "" {
		t.Fatalf("missing auth should be rejected token=%q ok=%v", token, ok)
	}

	req.Header.Set("Authorization", "Basic abc")
	if token, ok := shared.BearerToken(req); ok || token != "" {
		t.Fatalf("wrong auth scheme should be rejected token=%q ok=%v", token, ok)
	}

	req.Header.Set("Authorization", "Bearer   ")
	if token, ok := shared.BearerToken(req); ok || token != "" {
		t.Fatalf("empty bearer token should be rejected token=%q ok=%v", token, ok)
	}

	req.Header.Set("Authorization", "Bearer token-value ")
	if token, ok := shared.BearerToken(req); !ok || token != "token-value" {
		t.Fatalf("bearer token parsed token=%q ok=%v", token, ok)
	}
}

func TestReadMultipartUploadReadsFileAndFieldsExactly(t *testing.T) {
	req := readMultipartUploadRequest(t, "file", "hero.png", []byte("abcde"), map[string]string{
		"duration_ms":           "7000",
		"overlay_opacity_light": "0.2",
	})

	upload, err := shared.ReadMultipartUpload(req, "file", 5)
	if err != nil {
		t.Fatal(err)
	}
	if upload.Filename != "hero.png" || string(upload.Data) != "abcde" {
		t.Fatalf("multipart upload file mismatch: %#v data=%q", upload, upload.Data)
	}
	wantFields := map[string]string{"duration_ms": "7000", "overlay_opacity_light": "0.2"}
	if !reflect.DeepEqual(upload.Fields, wantFields) {
		t.Fatalf("multipart upload fields=%#v want %#v", upload.Fields, wantFields)
	}
}

func TestReadMultipartUploadRejectsExactMalformedInputs(t *testing.T) {
	req := readMultipartUploadRequest(t, "file", "hero.png", []byte("abcdef"), nil)
	upload, err := shared.ReadMultipartUpload(req, "file", 5)
	if !reflect.DeepEqual(upload, shared.MultipartUpload{}) {
		t.Fatalf("oversized upload should return zero upload, got %#v", upload)
	}
	assertSharedHTTPError(t, err, http.StatusBadRequest, "upload_file.validate.too_large")

	req = readMultipartUploadRequest(t, "note", "ignored.txt", []byte("abcde"), map[string]string{"title": "missing"})
	upload, err = shared.ReadMultipartUpload(req, "file", 5)
	if !reflect.DeepEqual(upload, shared.MultipartUpload{}) {
		t.Fatalf("missing file should return zero upload, got %#v", upload)
	}
	assertSharedHTTPError(t, err, http.StatusBadRequest, "upload_file.validate.required")

	req = httptest.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	upload, err = shared.ReadMultipartUpload(req, "file", 5)
	if !reflect.DeepEqual(upload, shared.MultipartUpload{}) {
		t.Fatalf("malformed upload should return zero upload, got %#v", upload)
	}
	assertSharedHTTPError(t, err, http.StatusBadRequest, "request.decode.invalid")
}

func TestReadMultipartUploadRejectsAmbiguousAndExcessivePartsExactly(t *testing.T) {
	requestWithParts := func(parts func(*multipart.Writer)) *http.Request {
		t.Helper()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		parts(writer)
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		return req
	}

	for _, tc := range []struct {
		name   string
		parts  func(*multipart.Writer)
		detail string
	}{
		{
			name: "duplicate file",
			parts: func(writer *multipart.Writer) {
				for index := 0; index < 2; index++ {
					part, err := writer.CreateFormFile("file", "hero.png")
					if err != nil {
						t.Fatal(err)
					}
					_, _ = part.Write([]byte("x"))
				}
			},
			detail: "upload_field.decode.conflict",
		},
		{
			name: "duplicate field",
			parts: func(writer *multipart.Writer) {
				_ = writer.WriteField("title", "first")
				_ = writer.WriteField("title", "second")
			},
			detail: "upload_field.decode.conflict",
		},
		{
			name: "oversized field",
			parts: func(writer *multipart.Writer) {
				_ = writer.WriteField("title", strings.Repeat("x", shared.MaxMultipartFieldBytes+1))
			},
			detail: "upload_field.decode.too_large",
		},
		{
			name: "too many fields",
			parts: func(writer *multipart.Writer) {
				for index := 0; index < shared.MaxMultipartParts+1; index++ {
					_ = writer.WriteField(fmt.Sprintf("field_%d", index), "x")
				}
			},
			detail: "upload_field.decode.exceeded",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upload, err := shared.ReadMultipartUpload(requestWithParts(tc.parts), "file", 5)
			if !reflect.DeepEqual(upload, shared.MultipartUpload{}) {
				t.Fatalf("rejected multipart returned upload %#v", upload)
			}
			assertSharedHTTPError(t, err, http.StatusBadRequest, tc.detail)
		})
	}
}

func readMultipartUploadRequest(t *testing.T, fileField, filename string, data []byte, fields map[string]string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func assertSharedHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	httpErr, ok := err.(util.HTTPError)
	if !ok {
		t.Fatalf("error type=%T detail=%v, want util.HTTPError{%d,%q}", err, err, status, detail)
	}
	if httpErr.Status != status || httpErr.Error() != detail {
		t.Fatalf("HTTPError=%#v, want status=%d detail=%q", httpErr, status, detail)
	}
}
