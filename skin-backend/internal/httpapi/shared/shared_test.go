package shared_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/util"

	"element-skin/backend/internal/httpapi/shared"
)

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
}
