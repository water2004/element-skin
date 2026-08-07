package microsoft_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMicrosoftHTTPClientProfile404ReturnsNilProfile(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" || req.URL.String() != "https://api.minecraftservices.com/minecraft/profile" {
			t.Fatalf("unexpected profile request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer mc_access" {
			t.Fatalf("missing bearer auth: %q", req.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	out, err := client.GetMinecraftProfile(context.Background(), "mc_access")
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("profile 404 should decode as nil profile, got %#v", out)
	}
}

func TestMicrosoftHTTPClientRejectsNonSuccessWithStatusAndBody(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("upstream failed")), Header: make(http.Header)}, nil
	})}}
	_, err := client.CheckGameOwnership(context.Background(), "mc_access")
	if err == nil || !strings.Contains(err.Error(), "status=502") || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("expected status/body error, got %v", err)
	}
}

func TestMicrosoftHTTPClientClassifiesUnauthorizedWithoutFlatteningOtherFailures(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("expired")), Header: make(http.Header)}, nil
	})}}
	_, err := client.CheckGameOwnership(context.Background(), "expired-access")
	var upstreamErr *microsoft.UpstreamHTTPError
	if !errors.As(err, &upstreamErr) || upstreamErr.StatusCode != http.StatusUnauthorized || upstreamErr.Body != "expired" || !microsoft.IsUnauthorized(err) {
		t.Fatalf("unauthorized classification mismatch: err=%#v upstream=%#v", err, upstreamErr)
	}
	if microsoft.IsUnauthorized(errors.New("network failed")) {
		t.Fatal("non-HTTP failure must not be classified as unauthorized")
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
