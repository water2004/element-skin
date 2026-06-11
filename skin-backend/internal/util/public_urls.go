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
		candidate := ProfileNameCandidate(base, i)
		if !exists(candidate) {
			return candidate, nil
		}
	}
	return "", HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}

func ProfileNameCandidate(base string, attempt int) string {
	suffix := ""
	if attempt > 0 {
		suffix = "_" + strconv.Itoa(attempt)
	}
	suffixRunes := []rune(suffix)
	if len(suffixRunes) >= 16 {
		return string(suffixRunes[len(suffixRunes)-16:])
	}
	baseRunes := []rune(base)
	limit := 16 - len(suffixRunes)
	if len(baseRunes) > limit {
		baseRunes = baseRunes[:limit]
	}
	return string(baseRunes) + suffix
}
