package settings

import "testing"

func TestSettingValueCoercesExactTypes(t *testing.T) {
	cases := []struct {
		key  string
		raw  string
		want any
	}{
		{key: "allow_register", raw: "true", want: true},
		{key: "allow_register", raw: "1", want: true},
		{key: "allow_register", raw: "false", want: false},
		{key: "max_texture_size", raw: "2048", want: 2048},
		{key: "max_texture_size", raw: "not-a-number", want: 1024},
		{key: "site_name", raw: "Exact Name", want: "Exact Name"},
	}
	for _, tt := range cases {
		if got := settingValue(tt.key, tt.raw); got != tt.want {
			t.Fatalf("settingValue(%q, %q)=%#v, want %#v", tt.key, tt.raw, got, tt.want)
		}
	}
}

func TestSettingValueFallbackServicesJSON(t *testing.T) {
	got := settingValue("fallback_services", `[{"session_url":"https://session","cache_ttl":30}]`)
	services := got.([]map[string]any)
	if len(services) != 1 || services[0]["session_url"] != "https://session" || services[0]["cache_ttl"].(float64) != 30 {
		t.Fatalf("fallback_services JSON did not decode exactly: %#v", got)
	}
	if got := settingValue("fallback_services", `{bad json}`); len(got.([]map[string]any)) != 0 {
		t.Fatalf("invalid fallback_services JSON should return empty list, got %#v", got)
	}
}
