package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

var SettingDefaults = map[string]string{
	"site_name":                    "皮肤站",
	"site_subtitle":                "简洁、高效、现代的 Minecraft 皮肤管理站",
	"require_invite":               "false",
	"allow_register":               "true",
	"enable_skin_library":          "true",
	"max_texture_size":             "1024",
	"footer_text":                  "",
	"filing_icp":                   "",
	"filing_icp_link":              "",
	"filing_mps":                   "",
	"filing_mps_link":              "",
	"profile_uuid_mode":            "random",
	"site_url":                     "",
	"api_url":                      "",
	"rate_limit_enabled":           "true",
	"rate_limit_auth_attempts":     "5",
	"rate_limit_auth_window":       "15",
	"enable_strong_password_check": "false",
	"jwt_expire_days":              "7",
	"microsoft_client_id":          "",
	"microsoft_client_secret":      "",
	"microsoft_redirect_uri":       "",
	"smtp_host":                    "",
	"smtp_port":                    "465",
	"smtp_user":                    "",
	"smtp_username":                "",
	"smtp_password":                "",
	"smtp_ssl":                     "true",
	"smtp_sender":                  "",
	"email_verify_enabled":         "false",
	"email_verify_ttl":             "300",
	"fallback_strategy":            "serial",
	"fallback_services":            "[]",
}

var settingsGroups = map[string][]string{
	"site": {
		"site_name", "allow_register", "require_invite", "enable_skin_library",
		"max_texture_size", "profile_uuid_mode", "site_url", "api_url",
		"site_subtitle", "footer_text", "filing_icp", "filing_icp_link", "filing_mps", "filing_mps_link",
	},
	"security":  {"rate_limit_enabled", "rate_limit_auth_attempts", "rate_limit_auth_window", "enable_strong_password_check"},
	"auth":      {"jwt_expire_days"},
	"microsoft": {"microsoft_client_id", "microsoft_client_secret", "microsoft_redirect_uri"},
	"email":     {"smtp_host", "smtp_port", "smtp_user", "smtp_username", "smtp_password", "smtp_ssl", "smtp_sender", "email_verify_enabled", "email_verify_ttl"},
	"fallback":  {"fallback_strategy", "fallback_services"},
}

type Settings struct {
	DB *database.DB
}

func (s Settings) GetGroup(ctx context.Context, group string) (map[string]any, error) {
	keys, ok := settingsGroups[group]
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "invalid settings group"}
	}
	out := map[string]any{}
	for _, key := range keys {
		raw, err := s.DB.GetSetting(ctx, key, SettingDefaults[key])
		if err != nil {
			return nil, err
		}
		out[key] = settingValue(key, raw)
	}
	if group == "fallback" {
		fallbacks, err := s.DB.ListFallbackEndpoints(ctx)
		if err != nil {
			return nil, err
		}
		out["fallbacks"] = fallbacks
	}
	return out, nil
}

func (s Settings) SaveGroup(ctx context.Context, group string, body map[string]any) error {
	keys, ok := settingsGroups[group]
	if !ok {
		return util.HTTPError{Status: 400, Detail: "invalid settings group"}
	}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	for key, value := range body {
		if !allowed[key] {
			continue
		}
		if key == "profile_uuid_mode" {
			mode := fmt.Sprint(value)
			if mode != "random" && mode != "offline" {
				return util.HTTPError{Status: 400, Detail: "invalid profile_uuid_mode"}
			}
		}
		if key == "smtp_password" && fmt.Sprint(value) == "" {
			continue
		}
		if key == "fallback_services" {
			normalized, err := ValidateFallbackServices(value)
			if err != nil {
				return err
			}
			b, err := json.Marshal(normalized)
			if err != nil {
				return err
			}
			value = string(b)
		}
		if err := s.DB.SetSetting(ctx, key, value); err != nil {
			return err
		}
	}
	if group == "fallback" {
		if raw, ok := body["fallbacks"]; ok {
			fallbacks, err := ValidateFallbackEndpoints(raw)
			if err != nil {
				return err
			}
			if err := s.DB.SaveFallbackEndpoints(ctx, fallbacks); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s Settings) Public(ctx context.Context, cfgSiteURL, cfgAPIURL string) (map[string]any, error) {
	siteName, err := s.DB.GetSetting(ctx, "site_name", SettingDefaults["site_name"])
	if err != nil {
		return nil, err
	}
	allow, err := s.DB.GetSetting(ctx, "allow_register", SettingDefaults["allow_register"])
	if err != nil {
		return nil, err
	}
	siteURL, _ := s.DB.GetSetting(ctx, "site_url", cfgSiteURL)
	apiURL, _ := s.DB.GetSetting(ctx, "api_url", cfgAPIURL)
	subtitle, _ := s.DB.GetSetting(ctx, "site_subtitle", SettingDefaults["site_subtitle"])
	enableLibrary, _ := s.DB.GetSetting(ctx, "enable_skin_library", SettingDefaults["enable_skin_library"])
	emailVerify, _ := s.DB.GetSetting(ctx, "email_verify_enabled", SettingDefaults["email_verify_enabled"])
	footer, _ := s.DB.GetSetting(ctx, "footer_text", SettingDefaults["footer_text"])
	icp, _ := s.DB.GetSetting(ctx, "filing_icp", SettingDefaults["filing_icp"])
	icpLink, _ := s.DB.GetSetting(ctx, "filing_icp_link", SettingDefaults["filing_icp_link"])
	mps, _ := s.DB.GetSetting(ctx, "filing_mps", SettingDefaults["filing_mps"])
	mpsLink, _ := s.DB.GetSetting(ctx, "filing_mps_link", SettingDefaults["filing_mps_link"])
	status := map[string]any{
		"session":  "https://sessionserver.mojang.com",
		"account":  "https://api.mojang.com",
		"services": "https://api.minecraftservices.com",
	}
	if primary, err := s.DB.GetPrimaryFallbackEndpoint(ctx); err == nil && primary != nil {
		status["session"] = primary["session_url"]
		status["account"] = primary["account_url"]
		status["services"] = primary["services_url"]
	}
	return map[string]any{
		"site_name":            siteName,
		"site_subtitle":        subtitle,
		"site_url":             util.NormalizePublicURL(siteURL),
		"api_url":              util.NormalizePublicURL(apiURL),
		"allow_register":       settingBool(allow),
		"enable_skin_library":  settingBool(enableLibrary),
		"email_verify_enabled": settingBool(emailVerify),
		"footer_text":          footer,
		"filing_icp":           icp,
		"filing_icp_link":      icpLink,
		"filing_mps":           mps,
		"filing_mps_link":      mpsLink,
		"mojang_status_urls":   status,
	}, nil
}

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

func settingValue(key, raw string) any {
	switch key {
	case "allow_register", "require_invite", "enable_skin_library", "rate_limit_enabled", "email_verify_enabled", "enable_strong_password_check", "smtp_ssl":
		return settingBool(raw)
	case "max_texture_size", "rate_limit_auth_attempts", "rate_limit_auth_window", "jwt_expire_days", "smtp_port", "email_verify_ttl":
		n, err := strconv.Atoi(raw)
		if err != nil {
			n, _ = strconv.Atoi(SettingDefaults[key])
		}
		return n
	case "fallback_services":
		var out []map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return []map[string]any{}
		}
		return out
	default:
		return raw
	}
}

func settingBool(raw string) bool {
	return raw == "true" || raw == "1"
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
