package settings

import (
	"context"

	"element-skin/backend/internal/util"
)

func (s Settings) Public(ctx context.Context, cfgSiteURL, cfgAPIURL string) (map[string]any, error) {
	siteName, err := s.Get(ctx, "site_name", SettingDefaults["site_name"])
	if err != nil {
		return nil, err
	}
	allow, err := s.Get(ctx, "allow_register", SettingDefaults["allow_register"])
	if err != nil {
		return nil, err
	}
	siteURL, err := s.Get(ctx, "site_url", cfgSiteURL)
	if err != nil {
		return nil, err
	}
	apiURL, err := s.Get(ctx, "api_url", cfgAPIURL)
	if err != nil {
		return nil, err
	}
	subtitle, err := s.Get(ctx, "site_subtitle", SettingDefaults["site_subtitle"])
	if err != nil {
		return nil, err
	}
	enableLibrary, err := s.Get(ctx, "enable_skin_library", SettingDefaults["enable_skin_library"])
	if err != nil {
		return nil, err
	}
	emailVerify, err := s.Get(ctx, "email_verify_enabled", SettingDefaults["email_verify_enabled"])
	if err != nil {
		return nil, err
	}
	footer, err := s.Get(ctx, "footer_text", SettingDefaults["footer_text"])
	if err != nil {
		return nil, err
	}
	icp, err := s.Get(ctx, "filing_icp", SettingDefaults["filing_icp"])
	if err != nil {
		return nil, err
	}
	icpLink, err := s.Get(ctx, "filing_icp_link", SettingDefaults["filing_icp_link"])
	if err != nil {
		return nil, err
	}
	mps, err := s.Get(ctx, "filing_mps", SettingDefaults["filing_mps"])
	if err != nil {
		return nil, err
	}
	mpsLink, err := s.Get(ctx, "filing_mps_link", SettingDefaults["filing_mps_link"])
	if err != nil {
		return nil, err
	}
	status := map[string]any{
		"session":  "https://sessionserver.mojang.com",
		"account":  "https://api.mojang.com",
		"services": "https://api.minecraftservices.com",
	}
	primary, err := s.DB.Fallbacks.PrimaryEndpoint(ctx)
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
