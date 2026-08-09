package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type failingYAMLValue struct{}

func (failingYAMLValue) MarshalYAML() (any, error) {
	return nil, errors.New("YAML value unavailable")
}

func TestRawConfigTypedReadersUseExactValuesAndFallbacks(t *testing.T) {
	fallbackStrings := []string{"fallback"}
	raw := rawConfig{
		"strings":      []string{"alpha", "beta"},
		"any_strings":  []any{"gamma", "delta"},
		"mixed":        []any{"valid", 42},
		"bool":         true,
		"true_string":  " true ",
		"false_string": "false",
		"invalid_bool": "enabled",
		"wrong_bool":   1,
	}
	if got := getStringSlice(raw, "strings", fallbackStrings); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("[]string value=%v", got)
	}
	gotStrings := getStringSlice(raw, "strings", fallbackStrings)
	gotStrings[0] = "mutated"
	if original := raw["strings"].([]string); original[0] != "alpha" {
		t.Fatalf("[]string reader aliased raw value: %v", original)
	}
	if got := getStringSlice(raw, "any_strings", fallbackStrings); !reflect.DeepEqual(got, []string{"gamma", "delta"}) {
		t.Fatalf("[]any string value=%v", got)
	}
	for name, got := range map[string][]string{
		"missing": getStringSlice(raw, "missing", fallbackStrings),
		"mixed":   getStringSlice(raw, "mixed", fallbackStrings),
		"wrong":   getStringSlice(rawConfig{"wrong": true}, "wrong", fallbackStrings),
	} {
		if !reflect.DeepEqual(got, fallbackStrings) {
			t.Fatalf("%s string fallback=%v want=%v", name, got, fallbackStrings)
		}
	}
	for name, tc := range map[string]struct {
		key      string
		fallback bool
		want     bool
	}{
		"native":         {key: "bool", fallback: false, want: true},
		"true string":    {key: "true_string", fallback: false, want: true},
		"false string":   {key: "false_string", fallback: true, want: false},
		"invalid string": {key: "invalid_bool", fallback: true, want: true},
		"wrong type":     {key: "wrong_bool", fallback: true, want: true},
		"missing":        {key: "missing_bool", fallback: false, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := getBool(raw, tc.key, tc.fallback); got != tc.want {
				t.Fatalf("getBool(%q)=%v want=%v", tc.key, got, tc.want)
			}
		})
	}
	if got := atoiDefault("not-an-int", 17); got != 17 {
		t.Fatalf("invalid integer fallback=%d want=17", got)
	}
}

func TestRawConfigPathResolutionAndWriteFailuresExactly(t *testing.T) {
	base := t.TempDir()
	absolute := filepath.Join(base, "absolute.pem")
	if got := resolveRelativePath(base, ""); got != "" {
		t.Fatalf("empty resolved path=%q", got)
	}
	if got := resolveRelativePath(base, absolute); got != absolute {
		t.Fatalf("absolute resolved path=%q want=%q", got, absolute)
	}
	wantRelative := filepath.Join(base, "keys", "private.pem")
	if got := resolveRelativePath(base, filepath.Join("keys", "private.pem")); got != wantRelative {
		t.Fatalf("relative resolved path=%q want=%q", got, wantRelative)
	}

	parentFile := filepath.Join(base, "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigFile(filepath.Join(parentFile, "config.yaml"), rawConfig{"site": map[string]any{"name": "test"}}); err == nil {
		t.Fatal("config write below regular file should fail")
	}
	if err := writeConfigFile(filepath.Join(base, "unsupported.yaml"), rawConfig{"unsupported": failingYAMLValue{}}); err == nil || err.Error() != "YAML value unavailable" {
		t.Fatalf("custom YAML marshaling error=%v", err)
	}
}
