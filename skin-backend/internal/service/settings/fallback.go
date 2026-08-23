package settings

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/util"
)

func ValidateFallbackEndpoints(value any) ([]fallback.Endpoint, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Object: "fallback", Operation: "configure", Reason: "invalid"}
	}
	out := make([]fallback.Endpoint, 0, len(raw))
	seenIDs := make(map[int]struct{}, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Object: "fallback", Operation: "configure", Reason: "invalid"}
		}
		normalized, err := normalizeFallbackMap(i+1, m)
		if err != nil {
			return nil, err
		}
		id, err := normalizeEndpointID(i+1, m["id"])
		if err != nil {
			return nil, err
		}
		if id > 0 {
			if _, exists := seenIDs[id]; exists {
				return nil, util.HTTPError{Status: 400, Object: "fallback_endpoint", Operation: "configure", Reason: "conflict", Params: map[string]any{"index": i + 1}}
			}
			seenIDs[id] = struct{}{}
		}
		out = append(out, fallback.Endpoint{
			ID:              id,
			Priority:        intValue(m["priority"], i+1),
			SessionURL:      normalized["session_url"].(string),
			AccountURL:      normalized["account_url"].(string),
			ServicesURL:     normalized["services_url"].(string),
			CacheTTL:        normalized["cache_ttl"].(int),
			SkinDomains:     normalized["skin_domains"].([]string),
			EnableProfile:   normalized["enable_profile"].(bool),
			EnableHasJoined: boolValue(valueOr(m["enable_hasjoined"], true)),
			EnableWhitelist: normalized["enable_whitelist"].(bool),
			Note:            strings.TrimSpace(fmt.Sprint(valueOr(m["note"], ""))),
		})
	}
	return out, nil
}

func normalizeEndpointID(idx int, value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	var id int64
	switch typed := value.(type) {
	case int:
		id = int64(typed)
	case int64:
		id = typed
	case float64:
		if typed != math.Trunc(typed) || typed > math.MaxInt32 {
			return 0, util.HTTPError{Status: 400, Object: "fallback_endpoint", Operation: "configure", Reason: "invalid", Params: map[string]any{"index": idx}}
		}
		id = int64(typed)
	default:
		return 0, util.HTTPError{Status: 400, Object: "fallback_endpoint", Operation: "configure", Reason: "invalid", Params: map[string]any{"index": idx}}
	}
	if id <= 0 || id > math.MaxInt32 {
		return 0, util.HTTPError{Status: 400, Object: "fallback_endpoint", Operation: "configure", Reason: "invalid", Params: map[string]any{"index": idx}}
	}
	return int(id), nil
}

func normalizeFallbackMap(idx int, m map[string]any) (map[string]any, error) {
	session := strings.TrimSpace(fmt.Sprint(m["session_url"]))
	account := strings.TrimSpace(fmt.Sprint(m["account_url"]))
	services := strings.TrimSpace(fmt.Sprint(m["services_url"]))
	if session == "" || account == "" || services == "" {
		return nil, util.HTTPError{Status: 400, Object: "fallback_url", Operation: "validate", Reason: "required", Params: map[string]any{"index": idx}}
	}
	ttl := intValue(m["cache_ttl"], 60)
	if ttl < 0 {
		return nil, util.HTTPError{Status: 400, Object: "fallback_cache_ttl", Operation: "validate", Reason: "invalid", Params: map[string]any{"index": idx}}
	}
	domains, err := normalizeDomains(idx, m["skin_domains"])
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"session_url":      session,
		"account_url":      account,
		"services_url":     services,
		"skin_domains":     domains,
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

func normalizeDomains(idx int, value any) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	var parts []string
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			part, ok := item.(string)
			if !ok {
				return nil, util.HTTPError{Status: 400, Object: "fallback_skin_domain", Operation: "validate", Reason: "invalid", Params: map[string]any{"index": idx}}
			}
			parts = append(parts, part)
		}
	case []string:
		parts = append(parts, v...)
	default:
		return nil, util.HTTPError{Status: 400, Object: "fallback_skin_domain", Operation: "validate", Reason: "invalid", Params: map[string]any{"index": idx}}
	}
	clean := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		clean = append(clean, part)
	}
	return clean, nil
}

func valueOr(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}
