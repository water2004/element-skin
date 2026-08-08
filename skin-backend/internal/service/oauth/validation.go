package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/url"
	"strings"
)

func validPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func validWebhookURL(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(raw) != raw || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	hostname := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return false
	}
	if ip := net.ParseIP(hostname); ip != nil && !publicWebhookIP(ip) {
		return false
	}
	return true
}

func publicWebhookIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil &&
		strings.TrimSpace(raw) == raw &&
		(u.Scheme == "https" || u.Scheme == "http") &&
		u.Host != "" &&
		u.User == nil &&
		u.Fragment == ""
}

func validClientStatus(status string) bool {
	switch status {
	case StatusPending, StatusActive, StatusRejected, StatusDisabled:
		return true
	default:
		return false
	}
}
