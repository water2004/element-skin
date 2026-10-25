package publicsite_test

func publicSettingsCacheFixture() map[string]any {
	return map[string]any{
		"site_name": "Cached Site", "site_subtitle": "Subtitle",
		"site_url": "https://site.example", "api_url": "https://api.example",
		"allow_register": true, "require_invite": false,
		"enable_skin_library": true, "email_verify_enabled": false,
		"footer_text": "Footer", "filing_icp": "", "filing_icp_link": "",
		"filing_mps": "", "filing_mps_link": "",
		"easter_eggs": map[string]any{"enabled": []any{}},
		"mojang_status_urls": map[string]any{
			"session": "https://session.example", "account": "https://account.example",
			"services": "https://services.example",
		},
		"email_suffix_policy": map[string]any{"mode": "disabled", "suffixes": []any{}},
	}
}
