package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesNestedYAMLConfig(t *testing.T) {
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
database:
  dsn: "postgresql://user:pass@localhost:5432/db?sslmode=disable"
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
	if cfg.DatabaseDSN != "postgresql://user:pass@localhost:5432/db?sslmode=disable" || cfg.MaxConnections != 23 {
		t.Fatalf("database fields not parsed: %#v", cfg)
	}
	if cfg.SiteURL != "https://skin.example.com" || cfg.APIURL != "https://skin.example.com/api" || cfg.ServerHost != "127.0.0.1" || cfg.ServerPort != "9001" {
		t.Fatalf("server fields not parsed: %#v", cfg)
	}
	if cfg.TexturesDir != "/data/textures" || cfg.CarouselDir != "/data/carousel" {
		t.Fatalf("storage directories not parsed: %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "keys", "private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "keys", "public.pem") {
		t.Fatalf("key paths should resolve relative to config file: %#v", cfg)
	}
}

func TestLoadEnvOverridesFileSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
database:
  dsn: "postgresql://file"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("DATABASE_DSN", "postgresql://env")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "env-secret-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("JWT_SECRET env should override file, got %q", cfg.JWTSecret)
	}
	if cfg.DatabaseDSN != "postgresql://env" {
		t.Fatalf("DATABASE_DSN env should override file, got %q", cfg.DatabaseDSN)
	}
}
