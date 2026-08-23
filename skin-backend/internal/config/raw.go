package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func setRaw(raw rawConfig, dotted string, value any) {
	cur := raw
	start := 0
	for i := 0; i <= len(dotted); i++ {
		if i != len(dotted) && dotted[i] != '.' {
			continue
		}
		key := dotted[start:i]
		if i == len(dotted) {
			cur[key] = value
			return
		}
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
		start = i + 1
	}
}

func writeConfigFile(path string, raw rawConfig) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

func (c *Config) resolveKeyPaths(baseDir string) {
	c.PrivateKeyPath = resolveRelativePath(baseDir, c.PrivateKeyPath)
	c.PublicKeyPath = resolveRelativePath(baseDir, c.PublicKeyPath)
	c.OIDCPrivateKeyPath = resolveRelativePath(baseDir, c.OIDCPrivateKeyPath)
	c.OIDCPublicKeyPath = resolveRelativePath(baseDir, c.OIDCPublicKeyPath)
}

func resolveRelativePath(baseDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func getString(raw rawConfig, dotted string, fallback string) string {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

func getInt(raw rawConfig, dotted string, fallback int) int {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		return atoiDefault(n, fallback)
	default:
		return fallback
	}
}

func getStringSlice(raw rawConfig, dotted string, fallback []string) []string {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch items := v.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == len(items) {
			return out
		}
		return fallback
	default:
		return fallback
	}
}

func getBool(raw rawConfig, dotted string, fallback bool) bool {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(b))
		if err == nil {
			return parsed
		}
		return fallback
	default:
		return fallback
	}
}

func lookup(raw rawConfig, dotted string) (any, bool) {
	cur := any(raw)
	start := 0
	for i := 0; i <= len(dotted); i++ {
		if i != len(dotted) && dotted[i] != '.' {
			continue
		}
		key := dotted[start:i]
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
		start = i + 1
	}
	return cur, true
}

func atoiDefault(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
