package settings

import (
	"strconv"

	"element-skin/backend/internal/util"
)

func isBooleanSetting(key string) bool {
	switch key {
	case "allow_register", "require_invite", "enable_skin_library", "rate_limit_enabled", "email_verify_enabled", "enable_strong_password_check", "smtp_ssl":
		return true
	default:
		return false
	}
}

func settingValue(key, raw string) any {
	if isBooleanSetting(key) {
		return settingBool(raw)
	}
	switch key {
	case "max_texture_size", "rate_limit_auth_attempts", "rate_limit_auth_window", "jwt_expire_days", "smtp_port", "email_verify_ttl", "fallback_probe_interval":
		n, err := strconv.Atoi(raw)
		if err != nil {
			n, _ = strconv.Atoi(SettingDefaults[key])
		}
		return n
	default:
		return raw
	}
}

func settingBool(raw string) bool {
	return raw == "true"
}

func booleanValue(key string, value any) (bool, error) {
	if enabled, ok := value.(bool); ok {
		return enabled, nil
	}
	return false, util.HTTPError{Status: 400, Object: "setting_value", Operation: "validate", Reason: "invalid", Params: map[string]any{"key": key, "expected_type": "boolean"}}
}
