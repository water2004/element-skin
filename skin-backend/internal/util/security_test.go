package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateOutboundURLBlocksUnsafeTargets(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/x",
		"http://192.168.1.1/x",
		"http://172.16.0.1/x",
		"http://[::1]/x",
		"http://0.0.0.0/x",
		"file:///etc/passwd",
		"ftp://internal/x",
		"",
	}
	for _, raw := range blocked {
		if err := ValidateOutboundURL(raw); err == nil {
			t.Fatalf("expected %q to be blocked", raw)
		}
	}
}

func TestValidateOutboundURLAllowsPublicIPLiteral(t *testing.T) {
	if err := ValidateOutboundURL("http://1.1.1.1/x"); err != nil {
		t.Fatalf("public IP literal should be allowed: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fakeClient(status int, contentLength int64, body []byte) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    status,
			ContentLength: contentLength,
			Body:          io.NopCloser(bytes.NewReader(body)),
			Header:        make(http.Header),
		}, nil
	})}
}

func TestDownloadTextureCaps(t *testing.T) {
	if _, err := DownloadTexture(fakeClient(200, 10_000, []byte("x")), "http://1.1.1.1/big.png", 1000); err == nil {
		t.Fatal("Content-Length over cap should be rejected")
	}
	if _, err := DownloadTexture(fakeClient(200, -1, bytes.Repeat([]byte("x"), 1200)), "http://1.1.1.1/stream.png", 1000); err == nil {
		t.Fatal("streamed body over cap should be rejected")
	}
	data, err := DownloadTexture(fakeClient(200, 6, []byte("abcdef")), "http://1.1.1.1/ok.png", 1024)
	if err != nil || string(data) != "abcdef" {
		t.Fatalf("small body got %q err=%v", data, err)
	}
	if _, err := DownloadTexture(fakeClient(404, -1, nil), "http://1.1.1.1/missing.png", 1024); err == nil {
		t.Fatal("non-200 should error")
	}
	if _, err := DownloadTexture(fakeClient(200, HardCapBytes+1, []byte("x")), "http://1.1.1.1/huge.png", 0); err == nil {
		t.Fatal("hard cap should apply when maxBytes <= 0")
	}
}

func TestInMemoryStateStore(t *testing.T) {
	store := NewInMemoryStateStore()
	store.Put("k", map[string]any{"user_id": "u1"}, time.Minute)
	got := store.Pop("k").(map[string]any)
	if got["user_id"] != "u1" {
		t.Fatalf("unexpected state value: %#v", got)
	}
	if store.Pop("k") != nil {
		t.Fatal("state pop should be one-shot")
	}
	if store.Pop("missing") != nil {
		t.Fatal("missing state should return nil")
	}
	store.Put("old", "v", 0)
	time.Sleep(10 * time.Millisecond)
	if store.Pop("old") != nil {
		t.Fatal("expired item should return nil")
	}
	store.Put("expired", "v", 0)
	time.Sleep(10 * time.Millisecond)
	store.Put("new", "v2", time.Minute)
	if store.Len() != 1 || store.Pop("new") != "v2" {
		t.Fatal("put should sweep expired items")
	}
}

func TestGenericInternalErrorsDoNotLeakDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	Error(rr, errors.New("database password leaked in stack trace"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["detail"] != InternalServerErrorDetail {
		t.Fatalf("internal error detail should be generic, got %#v", body)
	}
	if bytes.Contains(rr.Body.Bytes(), []byte("password")) {
		t.Fatalf("internal error response leaked original error: %s", rr.Body.String())
	}
}
