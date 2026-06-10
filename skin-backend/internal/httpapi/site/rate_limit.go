package site

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

func (h Handler) checkAuthRateLimit(w http.ResponseWriter, req *http.Request, scope string) bool {
	enabled, err := h.settings.Get(req.Context(), "rate_limit_enabled", settings.SettingDefaults["rate_limit_enabled"])
	if err != nil {
		util.Error(w, err)
		return false
	}
	if enabled != "true" {
		return true
	}
	limit, err := h.settings.Int(req.Context(), "rate_limit_auth_attempts", 5)
	if err != nil {
		util.Error(w, err)
		return false
	}
	windowMinutes, err := h.settings.Int(req.Context(), "rate_limit_auth_window", 15)
	if err != nil {
		util.Error(w, err)
		return false
	}
	result, err := h.redis.HitRateLimit(req.Context(), "auth:"+scope+":ip:"+clientIP(req), limit, time.Duration(windowMinutes)*time.Minute)
	if err != nil {
		util.Error(w, err)
		return false
	}
	if !result.Allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(result.RetryAfter))
		util.Error(w, util.HTTPError{Status: http.StatusTooManyRequests, Detail: "Too many requests, please try again later"})
		return false
	}
	return true
}

func clientIP(req *http.Request) string {
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return req.RemoteAddr
}

func retryAfterSeconds(d time.Duration) string {
	if d <= 0 {
		return "1"
	}
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
