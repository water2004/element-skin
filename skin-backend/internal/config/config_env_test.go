package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadEnvOverridesFileAndPersistsExactYAMLValues(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
  expire_days: 3
  access_expire_minutes: 10
keys:
  private_key: "file-private.pem"
  public_key: "file-public.pem"
oidc:
  private_key: "file-oidc-private.pem"
  public_key: "file-oidc-public.pem"
identity:
  encryption_key: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
database:
  host: "file-db"
  port: "5432"
  user: "file-user"
  password: "file-password"
  name: "file-db-name"
  sslmode: "disable"
  max_connections: 7
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
  host: "file-redis"
  port: "6379"
  password: "file-redis-password"
  public_cache_ttl_seconds: 12
  auth_cache_ttl_seconds: 13
cors:
  allow_origins:
    - "https://file.example"
  allow_credentials: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("JWT_EXPIRE_DAYS", "8")
	t.Setenv("JWT_ACCESS_EXPIRE_MINUTES", "35")
	t.Setenv("KEYS_PRIVATE_KEY", "env-private.pem")
	t.Setenv("KEYS_PUBLIC_KEY", "abs/env-public.pem")
	t.Setenv("OIDC_PRIVATE_KEY", "env-oidc-private.pem")
	t.Setenv("OIDC_PUBLIC_KEY", "abs/env-oidc-public.pem")
	t.Setenv("IDENTITY_ENCRYPTION_KEY", "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("DATABASE_HOST", "env-db")
	t.Setenv("DATABASE_PORT", "6543")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-password")
	t.Setenv("DATABASE_NAME", "env-db-name")
	t.Setenv("DATABASE_SSLMODE", "require")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "31")
	t.Setenv("WEBHOOK_WORKER_MAX_DATABASE_CONNECTIONS", "3")
	t.Setenv("WEBHOOK_WORKER_ACTIVE_INTERVAL_MS", "1750")
	t.Setenv("SERVER_SITE_URL", "https://env.example")
	t.Setenv("SERVER_API_URL", "https://env.example/api")
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "8100")
	t.Setenv("TEXTURES_DIRECTORY", "/env/textures")
	t.Setenv("CAROUSEL_DIRECTORY", "/env/carousel")
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "env-redis-password")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_KEY_PREFIX", "envprefix:")
	t.Setenv("REDIS_PUBLIC_CACHE_TTL_SECONDS", "220")
	t.Setenv("REDIS_AUTH_CACHE_TTL_SECONDS", "25")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://env.example, http://localhost:5173")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "env-secret-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("JWT_SECRET env should override file, got %q", cfg.JWTSecret)
	}
	if cfg.JWTExpireDays != 8 || cfg.AccessMinutes != 35 {
		t.Fatalf("jwt env should override file, got %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "env-private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "abs", "env-public.pem") {
		t.Fatalf("key path env should override and resolve exactly, got %#v", cfg)
	}
	if cfg.OIDCPrivateKeyPath != filepath.Join(dir, "env-oidc-private.pem") || cfg.OIDCPublicKeyPath != filepath.Join(dir, "abs", "env-oidc-public.pem") ||
		cfg.IdentityEncryptionKey != "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" {
		t.Fatalf("OIDC identity env mismatch: %#v", cfg)
	}
	if cfg.DatabaseHost != "env-db" || cfg.DatabasePort != "6543" || cfg.DatabaseUser != "env-user" ||
		cfg.DatabasePassword != "env-password" || cfg.DatabaseName != "env-db-name" || cfg.DatabaseSSLMode != "require" ||
		cfg.DatabaseDSN != "postgresql://env-user:env-password@env-db:6543/env-db-name?sslmode=require" {
		t.Fatalf("database env should override file and derive DSN, got %#v", cfg)
	}
	if cfg.MaxConnections != 31 || cfg.SiteURL != "https://env.example" || cfg.APIURL != "https://env.example/api" ||
		cfg.ServerHost != "0.0.0.0" || cfg.ServerPort != "8100" || cfg.TexturesDir != "/env/textures" || cfg.CarouselDir != "/env/carousel" {
		t.Fatalf("env should override file/defaults: %#v", cfg)
	}
	if cfg.WebhookWorkerMaxConnections != 3 || cfg.WebhookWorkerActiveIntervalMS != 1750 {
		t.Fatalf("webhook worker env mismatch: %#v", cfg)
	}
	if cfg.RedisHost != "127.0.0.1" || cfg.RedisPort != "6380" || cfg.RedisAddr != "127.0.0.1:6380" ||
		cfg.RedisPassword != "env-redis-password" || cfg.RedisDB != 3 || cfg.RedisKeyPrefix != "envprefix:" ||
		cfg.PublicCacheTTL != 220 || cfg.AuthCacheTTL != 25 {
		t.Fatalf("redis env should override file/defaults: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.CORSOrigins, []string{"https://env.example", "http://localhost:5173"}) || !cfg.CORSCredentials {
		t.Fatalf("cors env mismatch: origins=%#v credentials=%v", cfg.CORSOrigins, cfg.CORSCredentials)
	}

	var persisted rawConfig
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &persisted); err != nil {
		t.Fatal(err)
	}
	assertRawValue(t, persisted, "jwt.secret", "env-secret-abcdefghijklmnopqrstuvwxyz")
	assertRawValue(t, persisted, "jwt.expire_days", 8)
	assertRawValue(t, persisted, "keys.private_key", "env-private.pem")
	assertRawValue(t, persisted, "oidc.private_key", "env-oidc-private.pem")
	assertRawValue(t, persisted, "identity.encryption_key", "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	assertRawValue(t, persisted, "database.host", "env-db")
	assertRawValue(t, persisted, "database.port", "6543")
	assertRawValue(t, persisted, "database.user", "env-user")
	assertRawValue(t, persisted, "database.password", "env-password")
	assertRawValue(t, persisted, "database.name", "env-db-name")
	assertRawValue(t, persisted, "database.sslmode", "require")
	assertRawValue(t, persisted, "database.max_connections", 31)
	assertRawValue(t, persisted, "webhook_worker.max_database_connections", 3)
	assertRawValue(t, persisted, "webhook_worker.active_interval_ms", 1750)
	assertRawValue(t, persisted, "server.port", 8100)
	assertRawValue(t, persisted, "redis.host", "127.0.0.1")
	assertRawValue(t, persisted, "redis.port", "6380")
	assertRawValue(t, persisted, "redis.public_cache_ttl_seconds", 220)
	assertRawValue(t, persisted, "cors.allow_credentials", true)
	if got, _ := lookup(persisted, "cors.allow_origins"); !reflect.DeepEqual(got, []any{"https://env.example", "http://localhost:5173"}) {
		t.Fatalf("persisted cors.allow_origins=%#v", got)
	}
}

func TestLoadMissingFileWithCompleteEnvironmentCreatesExactConfig(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "generated.yaml")
	setCompleteConfigEnv(t)

	cfg, err := Load(missingPath)
	if err != nil {
		t.Fatalf("complete environment should create config: %v", err)
	}
	want := Config{
		DatabaseDSN:                   "postgresql://env-user:env-password@env-db:6543/env-db-name?sslmode=require",
		DatabaseHost:                  "env-db",
		DatabasePort:                  "6543",
		DatabaseUser:                  "env-user",
		DatabasePassword:              "env-password",
		DatabaseName:                  "env-db-name",
		DatabaseSSLMode:               "require",
		MaxConnections:                31,
		WebhookWorkerMaxConnections:   3,
		WebhookWorkerActiveIntervalMS: 1750,
		JWTSecret:                     "env-secret-abcdefghijklmnopqrstuvwxyz",
		JWTExpireDays:                 8,
		AccessMinutes:                 35,
		SiteURL:                       "https://env.example",
		APIURL:                        "https://env.example/api",
		ServerHost:                    "0.0.0.0",
		ServerPort:                    "8100",
		TexturesDir:                   "/env/textures",
		CarouselDir:                   "/env/carousel",
		RedisAddr:                     "127.0.0.1:6380",
		RedisHost:                     "127.0.0.1",
		RedisPort:                     "6380",
		RedisPassword:                 "env-redis-password",
		RedisDB:                       3,
		RedisKeyPrefix:                "envprefix:",
		PublicCacheTTL:                220,
		AuthCacheTTL:                  25,
		PrivateKeyPath:                filepath.Join(dir, "env-private.pem"),
		PublicKeyPath:                 filepath.Join(dir, "abs", "env-public.pem"),
		OIDCPrivateKeyPath:            filepath.Join(dir, "env-oidc-private.pem"),
		OIDCPublicKeyPath:             filepath.Join(dir, "abs", "env-oidc-public.pem"),
		IdentityEncryptionKey:         "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		CORSOrigins:                   []string{"https://env.example", "http://localhost:5173"},
		CORSCredentials:               false,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("generated config mismatch:\n got: %#v\nwant: %#v", cfg, want)
	}
	var persisted rawConfig
	b, err := os.ReadFile(missingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &persisted); err != nil {
		t.Fatal(err)
	}
	assertRawValue(t, persisted, "jwt.secret", "env-secret-abcdefghijklmnopqrstuvwxyz")
	assertRawValue(t, persisted, "jwt.expire_days", 8)
	assertRawValue(t, persisted, "jwt.access_expire_minutes", 35)
	assertRawValue(t, persisted, "database.host", "env-db")
	assertRawValue(t, persisted, "database.port", "6543")
	assertRawValue(t, persisted, "database.user", "env-user")
	assertRawValue(t, persisted, "database.password", "env-password")
	assertRawValue(t, persisted, "database.name", "env-db-name")
	assertRawValue(t, persisted, "database.sslmode", "require")
	assertRawValue(t, persisted, "database.max_connections", 31)
	assertRawValue(t, persisted, "webhook_worker.max_database_connections", 3)
	assertRawValue(t, persisted, "webhook_worker.active_interval_ms", 1750)
	assertRawValue(t, persisted, "server.port", 8100)
	assertRawValue(t, persisted, "redis.host", "127.0.0.1")
	assertRawValue(t, persisted, "redis.port", "6380")
	assertRawValue(t, persisted, "redis.password", "env-redis-password")
	assertRawValue(t, persisted, "cors.allow_credentials", false)
	if got, _ := lookup(persisted, "cors.allow_origins"); !reflect.DeepEqual(got, []any{"https://env.example", "http://localhost:5173"}) {
		t.Fatalf("persisted cors.allow_origins=%#v", got)
	}
}
