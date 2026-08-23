package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadParsesNestedYAMLConfig(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "abcdefghijklmnopqrstuvwxyz1234567890"
  expire_days: 11
  access_expire_minutes: 45
keys:
  private_key: "keys/private.pem"
  public_key: "keys/public.pem"
oidc:
  private_key: "keys/oidc-private.pem"
  public_key: "keys/oidc-public.pem"
identity:
  encryption_key: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
database:
  host: "localhost"
  port: "5432"
  user: "user"
  password: "pass"
  name: "db"
  sslmode: "disable"
  max_connections: 23
server:
  site_url: "https://skin.example.com"
  api_url: "https://skin.example.com/api"
  host: "127.0.0.1"
  port: 9001
textures:
  directory: "/data/textures"
carousel:
  directory: "/data/carousel"
redis:
  host: "redis"
  port: "6379"
  password: "password123"
  db: 2
  key_prefix: "custom:"
  public_cache_ttl_seconds: 120
  auth_cache_ttl_seconds: 15
cors:
  allow_origins:
    - "https://skin.example.com"
    - "http://localhost:5173"
  allow_credentials: false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "abcdefghijklmnopqrstuvwxyz1234567890" {
		t.Fatalf("JWTSecret was not loaded from nested YAML: %#v", cfg)
	}
	if cfg.JWTExpireDays != 11 || cfg.AccessMinutes != 45 {
		t.Fatalf("JWT expiry fields not parsed: %#v", cfg)
	}
	if cfg.DatabaseHost != "localhost" || cfg.DatabasePort != "5432" || cfg.DatabaseUser != "user" ||
		cfg.DatabasePassword != "pass" || cfg.DatabaseName != "db" || cfg.DatabaseSSLMode != "disable" ||
		cfg.DatabaseDSN != "postgresql://user:pass@localhost:5432/db?sslmode=disable" || cfg.MaxConnections != 23 {
		t.Fatalf("database fields not parsed: %#v", cfg)
	}
	if cfg.SiteURL != "https://skin.example.com" || cfg.APIURL != "https://skin.example.com/api" || cfg.ServerHost != "127.0.0.1" || cfg.ServerPort != "9001" {
		t.Fatalf("server fields not parsed: %#v", cfg)
	}
	if cfg.TexturesDir != "/data/textures" || cfg.CarouselDir != "/data/carousel" {
		t.Fatalf("storage directories not parsed: %#v", cfg)
	}
	if cfg.RedisHost != "redis" || cfg.RedisPort != "6379" || cfg.RedisAddr != "redis:6379" ||
		cfg.RedisPassword != "password123" || cfg.RedisDB != 2 || cfg.RedisKeyPrefix != "custom:" {
		t.Fatalf("redis fields not parsed: %#v", cfg)
	}
	if cfg.PublicCacheTTL != 120 || cfg.AuthCacheTTL != 15 {
		t.Fatalf("redis ttl fields not parsed: %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "keys", "private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "keys", "public.pem") {
		t.Fatalf("key paths should resolve relative to config file: %#v", cfg)
	}
	if cfg.OIDCPrivateKeyPath != filepath.Join(dir, "keys", "oidc-private.pem") || cfg.OIDCPublicKeyPath != filepath.Join(dir, "keys", "oidc-public.pem") ||
		cfg.IdentityEncryptionKey != "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" {
		t.Fatalf("OIDC and identity config mismatch: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.CORSOrigins, []string{"https://skin.example.com", "http://localhost:5173"}) || cfg.CORSCredentials {
		t.Fatalf("cors fields not parsed: origins=%#v credentials=%v", cfg.CORSOrigins, cfg.CORSCredentials)
	}
}

func TestLoadWithoutEnvironmentDoesNotRewriteExistingFile(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := minimalConfigYAML()
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("config without env overrides should not be rewritten:\n got %q\nwant %q", string(after), original)
	}
}
