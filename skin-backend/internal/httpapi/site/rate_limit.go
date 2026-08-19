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
		util.Error(w, util.HTTPError{Status: http.StatusTooManyRequests, Object: "rate_limit", Operation: "check", Reason: "exceeded"})
		return false
	}
	return true
}

func clientIP(req *http.Request) string {
	remote := remoteIP(req.RemoteAddr)
	if remote == nil || !trustedForwardingPeer(remote) {
		if remote != nil {
			return remote.String()
		}
		return req.RemoteAddr
	}
	forwarded := strings.Split(req.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := net.ParseIP(strings.TrimSpace(forwarded[index]))
		if candidate != nil {
			return candidate.String()
		}
	}
	return remote.String()
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(strings.TrimSpace(remoteAddr))
}

func trustedForwardingPeer(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
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
