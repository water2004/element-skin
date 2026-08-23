package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMissingFileWithoutCompleteEnvironmentFailsAndDoesNotWrite(t *testing.T) {
	clearConfigEnv(t)
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := Load(missingPath)
	if err == nil || !strings.HasPrefix(err.Error(), "missing required config fields: ") {
		t.Fatalf("missing config error=%v cfg=%#v; want missing required fields", err, cfg)
	}
	if !reflect.DeepEqual(cfg, Config{}) {
		t.Fatalf("missing incomplete config should return zero config, got %#v", cfg)
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("incomplete env should not create config file, stat err=%v", err)
	}
}

func TestLoadMalformedFileReturnsZeroConfigAndDoesNotRewrite(t *testing.T) {
	clearConfigEnv(t)
	malformedPath := filepath.Join(t.TempDir(), "malformed.yaml")
	malformed := []byte("jwt:\n  secret: [unterminated")
	if err := os.WriteFile(malformedPath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(malformedPath)
	if err == nil || !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("malformed config result: cfg=%#v err=%v; want zero config plus YAML error", got, err)
	}
	after, readErr := os.ReadFile(malformedPath)
	if readErr != nil || string(after) != string(malformed) {
		t.Fatalf("malformed config should not be rewritten: readErr=%v after=%q", readErr, string(after))
	}
}

func TestLoadRejectsInvalidConfigValuesWithoutRewrite(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := `
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
  expire_days: 7
  access_expire_minutes: 30
keys:
  private_key: "private.pem"
  public_key: "public.pem"
oidc:
  private_key: "oidc-private.pem"
  public_key: "oidc-public.pem"
identity:
  encryption_key: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
database:
  host: "localhost"
  port: "5432"
  user: "file-user"
  password: "file-password"
  name: "file-db"
  sslmode: "disable"
  max_connections: 0
server:
  site_url: "https://file.example"
  api_url: "https://file.example/api"
  host: "127.0.0.1"
  port: 9000
textures:
  directory: "/file/textures"
carousel:
  directory: "/file/carousel"
redis:
  host: "redis"
  port: "6379"
  password: "file-redis-password"
  db: 0
  key_prefix: "file:"
  public_cache_ttl_seconds: 60
  auth_cache_ttl_seconds: 30
cors:
  allow_origins:
    - "https://file.example"
  allow_credentials: true
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err == nil || err.Error() != "invalid config database.max_connections" {
		t.Fatalf("invalid config error=%v cfg=%#v; want exact database.max_connections error", err, cfg)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || string(after) != raw {
		t.Fatalf("invalid config should not be rewritten: readErr=%v after=%q", readErr, string(after))
	}
}

func TestLoadRejectsInvalidEnvironmentOverride(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(minimalConfigYAML()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_PORT", "not-a-port")

	cfg, err := Load(path)
	if err == nil || err.Error() != "invalid environment variable SERVER_PORT" {
		t.Fatalf("invalid env error=%v cfg=%#v; want exact SERVER_PORT error", err, cfg)
	}
}

func TestLoadRejectsInvalidWebhookWorkerBudgetExactly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		field     string
		wantError string
	}{
		{
			name:      "database connections",
			field:     "  max_database_connections: 0\n  active_interval_ms: 3000",
			wantError: "invalid config webhook_worker.max_database_connections",
		},
		{
			name:      "active interval",
			field:     "  max_database_connections: 2\n  active_interval_ms: 0",
			wantError: "invalid config webhook_worker.active_interval_ms",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			path := filepath.Join(t.TempDir(), "config.yaml")
			raw := minimalConfigYAML() + "\nwebhook_worker:\n" + tc.field + "\n"
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err == nil || err.Error() != tc.wantError {
				t.Fatalf("invalid webhook worker budget cfg=%#v err=%v want=%q", cfg, err, tc.wantError)
			}
		})
	}
}

func TestLoadRejectsOverflowingMaxConnectionsFromFileAndEnvironmentExactly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fileValue string
		envValue  string
		wantError string
	}{
		{
			name:      "file",
			fileValue: "4294967297",
			wantError: "invalid config database.max_connections",
		},
		{
			name:      "environment",
			fileValue: "10",
			envValue:  "4294967297",
			wantError: "invalid environment variable DATABASE_MAX_CONNECTIONS",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			path := filepath.Join(t.TempDir(), "config.yaml")
			raw := strings.Replace(minimalConfigYAML(), "max_connections: 10", "max_connections: "+tc.fileValue, 1)
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.envValue != "" {
				t.Setenv("DATABASE_MAX_CONNECTIONS", tc.envValue)
			}

			cfg, err := Load(path)
			if err == nil || err.Error() != tc.wantError {
				t.Fatalf("overflow result: cfg=%#v err=%v; want %q", cfg, err, tc.wantError)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != raw {
				t.Fatalf("overflowing config must not be rewritten: readErr=%v after=%q", readErr, string(after))
			}
		})
	}
}

func TestLoadRejectsCredentialedWildcardCORSExactly(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := strings.Replace(
		minimalConfigYAML(),
		"    - \"https://file.example\"\n  allow_credentials: true",
		"    - \"*\"\n  allow_credentials: true",
		1,
	)
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err == nil || err.Error() != "invalid config cors.allow_origins: wildcard is not allowed when credentials are enabled" {
		t.Fatalf("credentialed wildcard result: cfg=%#v err=%v", cfg, err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || string(after) != raw {
		t.Fatalf("invalid CORS config must not be rewritten: readErr=%v after=%q", readErr, string(after))
	}
}

func TestLoadRejectsMalformedPublicURLsAndCORSOriginsExactly(t *testing.T) {
	tests := []struct {
		name      string
		oldValue  string
		newValue  string
		wantError string
	}{
		{name: "site URL without host", oldValue: `site_url: "https://file.example"`, newValue: `site_url: "https:///missing-host"`, wantError: "invalid config server.site_url"},
		{name: "site URL with credentials", oldValue: `site_url: "https://file.example"`, newValue: `site_url: "https://user:secret@file.example"`, wantError: "invalid config server.site_url"},
		{name: "API URL with query", oldValue: `api_url: "https://file.example/api"`, newValue: `api_url: "https://file.example/api?tenant=one"`, wantError: "invalid config server.api_url"},
		{name: "API URL with fragment", oldValue: `api_url: "https://file.example/api"`, newValue: `api_url: "https://file.example/api#fragment"`, wantError: "invalid config server.api_url"},
		{name: "CORS origin with path", oldValue: `    - "https://file.example"`, newValue: `    - "https://file.example/app"`, wantError: "invalid config cors.allow_origins"},
		{name: "CORS origin with credentials", oldValue: `    - "https://file.example"`, newValue: `    - "https://user@file.example"`, wantError: "invalid config cors.allow_origins"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			path := filepath.Join(t.TempDir(), "config.yaml")
			raw := strings.Replace(minimalConfigYAML(), tc.oldValue, tc.newValue, 1)
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(path)
			if err == nil || err.Error() != tc.wantError || !reflect.DeepEqual(cfg, Config{}) {
				t.Fatalf("invalid URL result: cfg=%#v err=%v; want zero config and %q", cfg, err, tc.wantError)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != raw {
				t.Fatalf("invalid URL config must not be rewritten: readErr=%v after=%q", readErr, string(after))
			}
		})
	}
}
