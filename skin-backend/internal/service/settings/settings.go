package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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
	siteURL, err := s.DB.GetSetting(ctx, "site_url", cfgSiteURL)
	if err != nil {
		return nil, err
	}
	apiURL, err := s.DB.GetSetting(ctx, "api_url", cfgAPIURL)
	if err != nil {
		return nil, err
	}
	subtitle, err := s.DB.GetSetting(ctx, "site_subtitle", SettingDefaults["site_subtitle"])
	if err != nil {
		return nil, err
	}
	enableLibrary, err := s.DB.GetSetting(ctx, "enable_skin_library", SettingDefaults["enable_skin_library"])
	if err != nil {
		return nil, err
	}
	emailVerify, err := s.DB.GetSetting(ctx, "email_verify_enabled", SettingDefaults["email_verify_enabled"])
	if err != nil {
		return nil, err
	}
	footer, err := s.DB.GetSetting(ctx, "footer_text", SettingDefaults["footer_text"])
	if err != nil {
		return nil, err
	}
	icp, err := s.DB.GetSetting(ctx, "filing_icp", SettingDefaults["filing_icp"])
	if err != nil {
		return nil, err
	}
	icpLink, err := s.DB.GetSetting(ctx, "filing_icp_link", SettingDefaults["filing_icp_link"])
	if err != nil {
		return nil, err
	}
	mps, err := s.DB.GetSetting(ctx, "filing_mps", SettingDefaults["filing_mps"])
	if err != nil {
		return nil, err
	}
	mpsLink, err := s.DB.GetSetting(ctx, "filing_mps_link", SettingDefaults["filing_mps_link"])
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"session":  "https://sessionserver.mojang.com",
		"account":  "https://api.mojang.com",
		"services": "https://api.minecraftservices.com",
	}
	primary, err := s.DB.GetPrimaryFallbackEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	if primary != nil {
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
