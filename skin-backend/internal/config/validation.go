package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func validateRequiredConfig(cfg Config, raw rawConfig) error {
	required := []string{
		"database.host",
		"database.port",
		"database.user",
		"database.password",
		"database.name",
		"database.sslmode",
		"database.max_connections",
		"jwt.secret",
		"jwt.expire_days",
		"jwt.access_expire_minutes",
		"server.site_url",
		"server.api_url",
		"server.host",
		"server.port",
		"textures.directory",
		"carousel.directory",
		"redis.host",
		"redis.port",
		"redis.password",
		"redis.db",
		"redis.key_prefix",
		"redis.public_cache_ttl_seconds",
		"redis.auth_cache_ttl_seconds",
		"keys.private_key",
		"keys.public_key",
		"oidc.private_key",
		"oidc.public_key",
		"identity.encryption_key",
		"cors.allow_origins",
		"cors.allow_credentials",
	}
	var missing []string
	for _, field := range required {
		if _, ok := lookup(raw, field); !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config fields: %s", strings.Join(missing, ", "))
	}
	if _, configured := lookup(raw, "webhook_worker.max_database_connections"); configured && cfg.WebhookWorkerMaxConnections <= 0 {
		return fmt.Errorf("invalid config webhook_worker.max_database_connections")
	}
	if _, configured := lookup(raw, "webhook_worker.active_interval_ms"); configured && cfg.WebhookWorkerActiveIntervalMS <= 0 {
		return fmt.Errorf("invalid config webhook_worker.active_interval_ms")
	}
	if cfg.CORSCredentials && containsString(cfg.CORSOrigins, "*") {
		return fmt.Errorf("invalid config cors.allow_origins: wildcard is not allowed when credentials are enabled")
	}
	if !validHTTPBaseURL(cfg.SiteURL) {
		return fmt.Errorf("invalid config server.site_url")
	}
	if !validHTTPBaseURL(cfg.APIURL) {
		return fmt.Errorf("invalid config server.api_url")
	}
	for _, origin := range cfg.CORSOrigins {
		if origin != "*" && !validCORSOrigin(origin) {
			return fmt.Errorf("invalid config cors.allow_origins")
		}
	}
	for _, check := range []struct {
		field string
		ok    bool
	}{
		{field: "database.host", ok: cfg.DatabaseHost != ""},
		{field: "database.port", ok: atoiDefault(cfg.DatabasePort, 0) > 0},
		{field: "database.user", ok: cfg.DatabaseUser != ""},
		{field: "database.name", ok: cfg.DatabaseName != ""},
		{field: "database.sslmode", ok: cfg.DatabaseSSLMode != ""},
		{field: "database.max_connections", ok: cfg.MaxConnections > 0},
		{field: "jwt.secret", ok: cfg.JWTSecret != ""},
		{field: "jwt.expire_days", ok: cfg.JWTExpireDays > 0},
		{field: "jwt.access_expire_minutes", ok: cfg.AccessMinutes > 0},
		{field: "server.site_url", ok: cfg.SiteURL != ""},
		{field: "server.api_url", ok: cfg.APIURL != ""},
		{field: "server.host", ok: cfg.ServerHost != ""},
		{field: "server.port", ok: atoiDefault(cfg.ServerPort, 0) > 0},
		{field: "textures.directory", ok: cfg.TexturesDir != ""},
		{field: "carousel.directory", ok: cfg.CarouselDir != ""},
		{field: "redis.host", ok: cfg.RedisHost != ""},
		{field: "redis.port", ok: atoiDefault(cfg.RedisPort, 0) > 0},
		{field: "redis.db", ok: cfg.RedisDB >= 0},
		{field: "redis.key_prefix", ok: cfg.RedisKeyPrefix != ""},
		{field: "redis.public_cache_ttl_seconds", ok: cfg.PublicCacheTTL > 0},
		{field: "redis.auth_cache_ttl_seconds", ok: cfg.AuthCacheTTL > 0},
		{field: "keys.private_key", ok: cfg.PrivateKeyPath != ""},
		{field: "keys.public_key", ok: cfg.PublicKeyPath != ""},
		{field: "oidc.private_key", ok: cfg.OIDCPrivateKeyPath != ""},
		{field: "oidc.public_key", ok: cfg.OIDCPublicKeyPath != ""},
		{field: "identity.encryption_key", ok: cfg.IdentityEncryptionKey != ""},
		{field: "cors.allow_origins", ok: len(cfg.CORSOrigins) > 0},
	} {
		if !check.ok {
			return fmt.Errorf("invalid config %s", check.field)
		}
	}
	return nil
}

func validHTTPBaseURL(raw string) bool {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil &&
		(u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" &&
		u.User == nil &&
		u.RawQuery == "" &&
		!u.ForceQuery &&
		u.Fragment == ""
}

func validCORSOrigin(raw string) bool {
	if !validHTTPBaseURL(raw) {
		return false
	}
	u, _ := url.Parse(raw)
	return u.Path == "" && u.RawPath == ""
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func (c *Config) deriveConnectionStrings() {
	if c.DatabaseHost != "" && c.DatabasePort != "" && c.DatabaseUser != "" && c.DatabaseName != "" && c.DatabaseSSLMode != "" {
		c.DatabaseDSN = postgresDSN(c.DatabaseHost, c.DatabasePort, c.DatabaseUser, c.DatabasePassword, c.DatabaseName, c.DatabaseSSLMode)
	}
	if c.RedisHost != "" && c.RedisPort != "" {
		c.RedisAddr = net.JoinHostPort(c.RedisHost, c.RedisPort)
	}
}

func postgresDSN(host, port, user, password, name, sslMode string) string {
	dsn := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   name,
	}
	if password == "" {
		dsn.User = url.User(user)
	}
	query := dsn.Query()
	query.Set("sslmode", sslMode)
	dsn.RawQuery = query.Encode()
	return dsn.String()
}
