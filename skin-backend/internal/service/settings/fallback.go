package settings

import (
	"fmt"
	"strconv"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func ValidateFallbackEndpoints(value any) ([]database.FallbackEndpoint, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "fallbacks must be a list"}
	}
	out := make([]database.FallbackEndpoint, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "invalid fallback entry"}
		}
		normalized, err := normalizeFallbackMap(i+1, m)
		if err != nil {
			return nil, err
		}
		out = append(out, database.FallbackEndpoint{
			Priority:        intValue(m["priority"], i+1),
			SessionURL:      normalized["session_url"].(string),
			AccountURL:      normalized["account_url"].(string),
			ServicesURL:     normalized["services_url"].(string),
			CacheTTL:        normalized["cache_ttl"].(int),
			SkinDomains:     normalized["skin_domains"].(string),
			EnableProfile:   normalized["enable_profile"].(bool),
			EnableHasJoined: boolValue(valueOr(m["enable_hasjoined"], true)),
			EnableWhitelist: normalized["enable_whitelist"].(bool),
			Note:            strings.TrimSpace(fmt.Sprint(valueOr(m["note"], ""))),
		})
	}
	return out, nil
}

func ValidateFallbackServices(value any) ([]map[string]any, error) {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]map[string]any); ok {
			raw = make([]any, 0, len(typed))
			for _, item := range typed {
				raw = append(raw, item)
			}
		} else {
			return nil, util.HTTPError{Status: 400, Detail: "fallback_services must be a list"}
		}
	}
	out := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "fallback service must be an object"}
		}
		normalized, err := normalizeFallbackMap(i+1, m)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeFallbackMap(idx int, m map[string]any) (map[string]any, error) {
	session := strings.TrimSpace(fmt.Sprint(m["session_url"]))
	account := strings.TrimSpace(fmt.Sprint(m["account_url"]))
	services := strings.TrimSpace(fmt.Sprint(m["services_url"]))
	if session == "" || account == "" || services == "" {
		return nil, util.HTTPError{Status: 400, Detail: fmt.Sprintf("fallback[%d] urls are required", idx)}
	}
	ttl := intValue(m["cache_ttl"], 60)
	if ttl < 0 {
		return nil, util.HTTPError{Status: 400, Detail: fmt.Sprintf("fallback[%d] cache_ttl must be non-negative", idx)}
	}
	return map[string]any{
		"session_url":      session,
		"account_url":      account,
		"services_url":     services,
		"skin_domains":     normalizeDomains(m["skin_domains"]),
		"cache_ttl":        ttl,
		"enable_profile":   boolValue(valueOr(m["enable_profile"], true)),
		"enable_whitelist": boolValue(valueOr(m["enable_whitelist"], false)),
	}, nil
}

func intValue(v any, fallback int) int {
	if v == nil || fmt.Sprint(v) == "" {
		return fallback
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(x)
		if err == nil {
			return n
		}
	}
	return fallback
}

func boolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return settingBool(x)
	case int:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}

func normalizeDomains(value any) string {
	var parts []string
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
	case []string:
		parts = append(parts, v...)
	case string:
		parts = strings.Split(v, ",")
	}
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, ",")
}

func valueOr(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}
