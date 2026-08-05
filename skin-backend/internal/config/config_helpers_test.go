package config

import (
	"reflect"
	"testing"
)

func assertRawValue(t *testing.T, raw rawConfig, dotted string, want any) {
	t.Helper()
	got, ok := lookup(raw, dotted)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("raw %s=%#v,%v; want %#v,true", dotted, got, ok, want)
	}
}

func minimalConfigYAML() string {
	return `
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
  max_connections: 10
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
}

func setCompleteConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("JWT_EXPIRE_DAYS", "8")
	t.Setenv("JWT_ACCESS_EXPIRE_MINUTES", "35")
	t.Setenv("KEYS_PRIVATE_KEY", "env-private.pem")
	t.Setenv("KEYS_PUBLIC_KEY", "/abs/env-public.pem")
	t.Setenv("OIDC_PRIVATE_KEY", "env-oidc-private.pem")
	t.Setenv("OIDC_PUBLIC_KEY", "/abs/env-oidc-public.pem")
	t.Setenv("IDENTITY_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("DATABASE_HOST", "env-db")
	t.Setenv("DATABASE_PORT", "6543")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-password")
	t.Setenv("DATABASE_NAME", "env-db-name")
	t.Setenv("DATABASE_SSLMODE", "require")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "31")
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
	t.Setenv("CORS_ALLOW_CREDENTIALS", "false")
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_HOST",
		"DATABASE_PORT",
		"DATABASE_USER",
		"DATABASE_PASSWORD",
		"DATABASE_NAME",
		"DATABASE_SSLMODE",
		"DATABASE_MAX_CONNECTIONS",
		"JWT_SECRET",
		"JWT_EXPIRE_DAYS",
		"JWT_ACCESS_EXPIRE_MINUTES",
		"KEYS_PRIVATE_KEY",
		"KEYS_PUBLIC_KEY",
		"OIDC_PRIVATE_KEY",
		"OIDC_PUBLIC_KEY",
		"IDENTITY_ENCRYPTION_KEY",
		"SERVER_SITE_URL",
		"SERVER_API_URL",
		"SERVER_HOST",
		"SERVER_PORT",
		"TEXTURES_DIRECTORY",
		"CAROUSEL_DIRECTORY",
		"REDIS_HOST",
		"REDIS_PORT",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_KEY_PREFIX",
		"REDIS_PUBLIC_CACHE_TTL_SECONDS",
		"REDIS_AUTH_CACHE_TTL_SECONDS",
		"CORS_ALLOW_ORIGINS",
		"CORS_ALLOW_CREDENTIALS",
	} {
		t.Setenv(key, "")
	}
}
