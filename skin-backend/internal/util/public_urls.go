package util

import (
	"net/url"
	"strconv"
	"strings"
)

func NormalizePublicURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func ValidProfileName(name string) bool {
	if name == "" || len(name) > 16 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func GenerateUniqueProfileName(base string, exists func(string) bool, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = 100
	}
	for i := 0; i < maxAttempts; i++ {
		candidate := base
		if i > 0 {
			candidate = base + "_" + strconv.Itoa(i)
		}
		if !exists(candidate) {
			return candidate, nil
		}
	}
	return "", HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}
