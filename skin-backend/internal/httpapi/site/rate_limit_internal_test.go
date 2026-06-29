package site

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
)

func TestCheckAuthRateLimitFailsClosedOnEachSettingsReadError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failKey string
		calls   int
	}{
		{name: "enabled setting", failKey: "rate_limit_enabled", calls: 1},
		{name: "attempt limit", failKey: "rate_limit_auth_attempts", calls: 2},
		{name: "window", failKey: "rate_limit_auth_window", calls: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &settingsReadFailStore{
				Store:   redisstore.NewMemoryStore(),
				failKey: tc.failKey,
			}
			h := Handler{
				redis:    store,
				settings: settings.Settings{Redis: store},
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/auth/login", nil)

			if allowed := h.checkAuthRateLimit(rec, req, "login"); allowed {
				t.Fatal("settings read failure must reject the authentication attempt")
			}
			if rec.Code != 500 || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
				t.Fatalf("response status=%d body=%q; want exact internal error", rec.Code, rec.Body.String())
			}
			if store.settingCalls != tc.calls || store.hitCalls != 0 {
				t.Fatalf("calls settings=%d rate-limit=%d; want %d and 0", store.settingCalls, store.hitCalls, tc.calls)
			}
		})
	}
}

func TestRateLimitAddressAndRetryHelpersReturnExactValues(t *testing.T) {
	for _, tc := range []struct {
		name       string
		forwarded  string
		remoteAddr string
		want       string
	}{
		{name: "first forwarded address", forwarded: " 203.0.113.7 , 198.51.100.2", remoteAddr: "10.0.0.1:1234", want: "203.0.113.7"},
		{name: "empty forwarded first value", forwarded: " , 198.51.100.2", remoteAddr: "10.0.0.1:1234", want: "10.0.0.1"},
		{name: "IPv6 host and port", remoteAddr: "[2001:db8::8]:4321", want: "2001:db8::8"},
		{name: "address without port", remoteAddr: "unix-client", want: "unix-client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := clientIP(req); got != tc.want {
				t.Fatalf("clientIP()=%q; want %q", got, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		duration time.Duration
		want     string
	}{
		{duration: -time.Second, want: "1"},
		{duration: 0, want: "1"},
		{duration: time.Nanosecond, want: "1"},
		{duration: time.Second, want: "1"},
		{duration: time.Second + time.Nanosecond, want: "2"},
		{duration: 90 * time.Second, want: "90"},
	} {
		if got := retryAfterSeconds(tc.duration); got != tc.want {
			t.Fatalf("retryAfterSeconds(%s)=%q; want %q", tc.duration, got, tc.want)
		}
	}
}

type settingsReadFailStore struct {
	redisstore.Store
	failKey      string
	settingCalls int
	hitCalls     int
}

func (s *settingsReadFailStore) GetSetting(_ context.Context, key string) (string, error) {
	s.settingCalls++
	if key == s.failKey {
		return "", errors.New("settings unavailable")
	}
	switch key {
	case "rate_limit_enabled":
		return "true", nil
	case "rate_limit_auth_attempts":
		return "5", nil
	case "rate_limit_auth_window":
		return "15", nil
	default:
		return "", redisstore.ErrCacheMiss
	}
}

func (s *settingsReadFailStore) HitRateLimit(context.Context, string, int, time.Duration) (redisstore.RateLimitResult, error) {
	s.hitCalls++
	return redisstore.RateLimitResult{Allowed: true}, nil
}
