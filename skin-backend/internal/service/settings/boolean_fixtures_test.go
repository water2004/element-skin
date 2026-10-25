package settings_test

var booleanSettingFields = []struct {
	group       string
	key         string
	companion   string
	original    any
	replacement any
}{
	{"site", "allow_register", "site_name", "Original Site", "Changed Site"},
	{"site", "require_invite", "site_name", "Original Site", "Changed Site"},
	{"site", "enable_skin_library", "site_name", "Original Site", "Changed Site"},
	{"security", "rate_limit_enabled", "rate_limit_auth_attempts", 5, 99},
	{"security", "enable_strong_password_check", "rate_limit_auth_attempts", 5, 99},
	{"email", "email_verify_enabled", "smtp_host", "original.example", "changed.example"},
	{"email", "smtp_ssl", "smtp_host", "original.example", "changed.example"},
}

var invalidBooleanValues = []struct {
	name  string
	value any
}{
	{"string_true", "true"},
	{"string_false", "false"},
	{"string_one", "1"},
	{"string_zero", "0"},
	{"uppercase", "TRUE"},
	{"whitespace", " true "},
	{"empty", ""},
	{"integer_one", 1},
	{"integer_zero", 0},
	{"decimal", float64(1)},
	{"null", nil},
	{"array", []any{}},
	{"object", map[string]any{}},
}

var fallbackBooleanFields = []string{"enable_profile", "enable_hasjoined", "enable_whitelist"}

func booleanFallbackEndpoint() map[string]any {
	return map[string]any{
		"session_url":      "https://session.example",
		"account_url":      "https://account.example",
		"services_url":     "https://services.example",
		"enable_profile":   true,
		"enable_hasjoined": true,
		"enable_whitelist": true,
	}
}
