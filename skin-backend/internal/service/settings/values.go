package settings

import (
	"encoding/json"
	"strconv"
)

func settingValue(key, raw string) any {
	switch key {
	case "allow_register", "require_invite", "enable_skin_library", "rate_limit_enabled", "email_verify_enabled", "enable_strong_password_check", "smtp_ssl":
		return settingBool(raw)
	case "max_texture_size", "rate_limit_auth_attempts", "rate_limit_auth_window", "jwt_expire_days", "smtp_port", "email_verify_ttl", "fallback_probe_interval":
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
	case "easter_eggs_enabled":
		var out []string
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return []string{}
		}
		return out
	default:
		return raw
	}
}

func settingBool(raw string) bool {
	return raw == "true" || raw == "1"
}
