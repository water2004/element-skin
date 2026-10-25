package settings

import "testing"

func TestSettingValueCoercesExactTypes(t *testing.T) {
	cases := []struct {
		key  string
		raw  string
		want any
	}{
		{key: "allow_register", raw: "true", want: true},
		{key: "allow_register", raw: "1", want: false},
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
