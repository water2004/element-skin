package util

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDownloadTextureExactSuccessStatusAndSizeLimitsByFile(t *testing.T) {
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
	if _, err := DownloadTexture(fileFakeClient(200, HardCapBytes+1, []byte("x")), "http://1.1.1.1/huge.png", 0); err == nil {
		t.Fatal("hard cap should apply when maxBytes <= 0")
	}
}

func fileFakeClient(status int, contentLength int64, body []byte) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    status,
			ContentLength: contentLength,
			Body:          io.NopCloser(bytes.NewReader(body)),
			Header:        make(http.Header),
		}, nil
	})}
}
