package util

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONAndErrorResponsesAreExact(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]any{"ok": true})
	if rec.Code != http.StatusCreated || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("JSON response mismatch: status=%d content-type=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "required"})
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"required\"}}\n" {
		t.Fatalf("HTTPError response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, YggError{Status: http.StatusForbidden, Code: "ForbiddenOperationException", Message: "Invalid token."})
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":\"ForbiddenOperationException\",\"errorMessage\":\"Invalid token.\"}\n" {
		t.Fatalf("Ygg HTTPError response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, errors.New("database password leaked"))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("generic error should be converged: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestDownloadTextureExactSuccessStatusAndSizeLimits(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/ok":
			return textureResponse(http.StatusOK, "abcde", 5), nil
		case "/too-large-header":
			return textureResponse(http.StatusOK, "abcdef", 6), nil
		case "/too-large-body":
			return textureResponse(http.StatusOK, "abcdef", -1), nil
		default:
			return textureResponse(http.StatusNotFound, "missing", -1), nil
		}
	})}

	data, err := DownloadTexture(client, "https://93.184.216.34/ok", 5)
	if err != nil || string(data) != "abcde" {
		t.Fatalf("DownloadTexture success mismatch: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/missing", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("non-200 should reject: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/too-large-header", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "texture too large") {
		t.Fatalf("large content-length should reject: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/too-large-body", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "texture too large") {
		t.Fatalf("large body should reject: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "http://127.0.0.1/ok", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "unsafe outbound URL") {
		t.Fatalf("unsafe URL should reject before HTTP request: data=%q err=%v", data, err)
	}
}

func textureResponse(status int, body string, contentLength int64) *http.Response {
	return &http.Response{
		StatusCode:    status,
		ContentLength: contentLength,
		Body:          io.NopCloser(strings.NewReader(body)),
		Header:        make(http.Header),
	}
}
