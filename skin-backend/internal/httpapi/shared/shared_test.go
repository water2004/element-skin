package shared_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"element-skin/backend/internal/util"

	"element-skin/backend/internal/httpapi/shared"
)

func TestRequestContextAndValueHelpersPreserveExactValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := shared.CurrentUserID(req); got != "" {
		t.Fatalf("request without auth context user ID=%q, want empty", got)
	}
	if shared.CurrentUserIsSuperAdmin(req) {
		t.Fatal("request without auth context must not be super admin")
	}

	req = req.WithContext(shared.WithUser(context.Background(), "user-123", true, true))
	if got := shared.CurrentUserID(req); got != "user-123" {
		t.Fatalf("context user ID=%q, want user-123", got)
	}
	if !shared.CurrentUserIsSuperAdmin(req) {
		t.Fatal("explicit super-admin flag should be preserved")
	}

	req = req.WithContext(shared.WithUser(context.Background(), "user-456", true))
	if shared.CurrentUserIsSuperAdmin(req) {
		t.Fatal("omitted super-admin flag must default to false")
	}

	value := map[string]any{"enabled": true}
	if got := shared.AsMap(value); !reflect.DeepEqual(got, value) {
		t.Fatalf("AsMap returned %#v, want %#v", got, value)
	}
	if got := shared.AsMap("not-a-map"); got != nil {
		t.Fatalf("AsMap(non-map)=%#v, want nil", got)
	}
	if got := shared.AsMap(nil); got != nil {
		t.Fatalf("AsMap(nil)=%#v, want nil", got)
	}
	if got := shared.AsString("exact"); got != "exact" {
		t.Fatalf("AsString(string)=%q, want exact", got)
	}
	if got := shared.AsString(123); got != "" {
		t.Fatalf("AsString(non-string)=%q, want empty", got)
	}
	if got := shared.ValueOrAny(nil, "fallback"); got != "fallback" {
		t.Fatalf("ValueOrAny(nil)=%#v, want fallback", got)
	}
	if got := shared.ValueOrAny(false, true); got != false {
		t.Fatalf("ValueOrAny(false)=%#v, want false", got)
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
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestPublicBoolAndValidation(t *testing.T) {
	cases := []struct {
		name  string
		input any
		valid bool
		value bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, true, false},
		{"float one", float64(1), true, true},
		{"float zero", float64(0), true, false},
		{"float invalid", float64(2), false, true},
		{"int one", 1, true, true},
		{"int zero", 0, true, false},
		{"int invalid", 2, false, true},
		{"string true", "true", true, true},
		{"string false", "false", true, false},
		{"string one", "1", true, true},
		{"string zero", "0", true, false},
		{"string invalid", "yes", false, false},
		{"unknown", []string{"true"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shared.ValidPublicValue(tc.input); got != tc.valid {
				t.Fatalf("shared.ValidPublicValue(%#v)=%v, want %v", tc.input, got, tc.valid)
			}
			if got := shared.PublicBool(tc.input); got != tc.value {
				t.Fatalf("shared.PublicBool(%#v)=%v, want %v", tc.input, got, tc.value)
			}
		})
	}
}

func TestCursorCreatedHashParsesExactKeys(t *testing.T) {
	cursor := util.EncodeCursor(map[string]any{"last_created_at": int64(12345), "last_skin_hash": "abc"})
	created, hash, err := shared.CursorCreatedHash(cursor, "last_skin_hash")
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || *created != 12345 || hash != "abc" {
		t.Fatalf("cursor parsed to created=%v hash=%q", created, hash)
	}

	created, hash, err = shared.CursorCreatedHash("", "last_skin_hash")
	if err != nil || created != nil || hash != "" {
		t.Fatalf("empty cursor should return nil cursor values, got created=%v hash=%q err=%v", created, hash, err)
	}

	if _, _, err := shared.CursorCreatedHash("not-base64", "last_skin_hash"); err == nil {
		t.Fatal("invalid cursor should return an error")
	}
	for _, payload := range []map[string]any{
		{"last_created_at": 1.5, "last_skin_hash": "abc"},
		{"last_created_at": -1, "last_skin_hash": "abc"},
		{"last_created_at": 1, "last_skin_hash": ""},
		{"last_created_at": 1},
	} {
		if _, _, err := shared.CursorCreatedHash(util.EncodeCursor(payload), "last_skin_hash"); err == nil {
			t.Fatalf("malformed cursor payload should reject: %#v", payload)
		}
	}
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

func TestMultipartFileBytesReadsExactFieldAndRejectsTooLarge(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "skin.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "abcde"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024); err != nil {
		t.Fatal(err)
	}
	data, err := shared.MultipartFileBytes(req, "file", 5)
	if err != nil || string(data) != "abcde" {
		t.Fatalf("shared.MultipartFileBytes exact read mismatch: data=%q err=%v", data, err)
	}

	body = &bytes.Buffer{}
	writer = multipart.NewWriter(body)
	part, err = writer.CreateFormFile("file", "skin.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024); err != nil {
		t.Fatal(err)
	}
	if data, err := shared.MultipartFileBytes(req, "file", 5); err == nil || string(data) != "" || !bytes.Contains([]byte(err.Error()), []byte("File too large")) {
		t.Fatalf("oversized upload should reject: data=%q err=%v", data, err)
	}

	body = &bytes.Buffer{}
	writer = multipart.NewWriter(body)
	if err := writer.WriteField("note", "missing file"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024); err != nil {
		t.Fatal(err)
	}
	if data, err := shared.MultipartFileBytes(req, "file", 5); err == nil || data != nil || err.Error() != "file is required" {
		t.Fatalf("missing upload field should return exact contract: data=%q err=%v", data, err)
	}
}
