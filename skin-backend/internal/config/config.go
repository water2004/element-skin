package config

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DatabaseDSN                   string
	DatabaseHost                  string
	DatabasePort                  string
	DatabaseUser                  string
	DatabasePassword              string
	DatabaseName                  string
	DatabaseSSLMode               string
	MaxConnections                int32
	WebhookWorkerMaxConnections   int32
	WebhookWorkerActiveIntervalMS int
	JWTSecret                     string
	JWTExpireDays                 int
	AccessMinutes                 int
	SiteURL                       string
	APIURL                        string
	ServerHost                    string
	ServerPort                    string
	TexturesDir                   string
	CarouselDir                   string
	RedisAddr                     string
	RedisHost                     string
	RedisPort                     string
	RedisPassword                 string
	RedisDB                       int
	RedisKeyPrefix                string
	PublicCacheTTL                int
	AuthCacheTTL                  int
	PrivateKeyPath                string
	PublicKeyPath                 string
	OIDCPrivateKeyPath            string
	OIDCPublicKeyPath             string
	IdentityEncryptionKey         string
	CORSOrigins                   []string
	CORSCredentials               bool
}

type rawConfig = map[string]any

func Load(path string) (Config, error) {
	cfg := Config{}
	raw := rawConfig{}
	writeConfig := false
	if b, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return cfg, err
		}
		cfg.apply(raw)
	} else if errors.Is(err, os.ErrNotExist) {
		writeConfig = true
	} else {
		return cfg, err
	}
	envChanged, err := applyEnvOverrides(&cfg, raw)
	if err != nil {
		return Config{}, err
	}
	if envChanged {
		writeConfig = true
	}
	cfg.deriveConnectionStrings()
	if err := validateRequiredConfig(cfg, raw); err != nil {
		return Config{}, err
	}
	if writeConfig {
		if err := writeConfigFile(path, raw); err != nil {
			return cfg, err
		}
	}
	cfg.resolveKeyPaths(filepath.Dir(path))
	return cfg, nil
}

func (c *Config) apply(raw rawConfig) {
	c.DatabaseHost = getString(raw, "database.host", c.DatabaseHost)
	c.DatabasePort = getString(raw, "database.port", c.DatabasePort)
	c.DatabaseUser = getString(raw, "database.user", c.DatabaseUser)
	c.DatabasePassword = getString(raw, "database.password", c.DatabasePassword)
	c.DatabaseName = getString(raw, "database.name", c.DatabaseName)
	c.DatabaseSSLMode = getString(raw, "database.sslmode", c.DatabaseSSLMode)
	if n := getInt(raw, "database.max_connections", int(c.MaxConnections)); n > 0 && n <= math.MaxInt32 {
		c.MaxConnections = int32(n)
	}
	if n := getInt(raw, "webhook_worker.max_database_connections", int(c.WebhookWorkerMaxConnections)); n > 0 && n <= math.MaxInt32 {
		c.WebhookWorkerMaxConnections = int32(n)
	}
	if n := getInt(raw, "webhook_worker.active_interval_ms", c.WebhookWorkerActiveIntervalMS); n > 0 {
		c.WebhookWorkerActiveIntervalMS = n
	}
	c.JWTSecret = getString(raw, "jwt.secret", c.JWTSecret)
	c.JWTExpireDays = getInt(raw, "jwt.expire_days", c.JWTExpireDays)
	c.AccessMinutes = getInt(raw, "jwt.access_expire_minutes", c.AccessMinutes)
	c.SiteURL = getString(raw, "server.site_url", c.SiteURL)
	c.APIURL = getString(raw, "server.api_url", c.APIURL)
	c.ServerHost = getString(raw, "server.host", c.ServerHost)
	if n := getInt(raw, "server.port", 0); n > 0 {
		c.ServerPort = strconv.Itoa(n)
	}
	c.TexturesDir = getString(raw, "textures.directory", c.TexturesDir)
	c.CarouselDir = getString(raw, "carousel.directory", c.CarouselDir)
	c.RedisHost = getString(raw, "redis.host", c.RedisHost)
	c.RedisPort = getString(raw, "redis.port", c.RedisPort)
	c.RedisPassword = getString(raw, "redis.password", c.RedisPassword)
	if n := getInt(raw, "redis.db", c.RedisDB); n >= 0 {
		c.RedisDB = n
	}
	c.RedisKeyPrefix = getString(raw, "redis.key_prefix", c.RedisKeyPrefix)
	if n := getInt(raw, "redis.public_cache_ttl_seconds", c.PublicCacheTTL); n > 0 {
		c.PublicCacheTTL = n
	}
	if n := getInt(raw, "redis.auth_cache_ttl_seconds", c.AuthCacheTTL); n > 0 {
		c.AuthCacheTTL = n
	}
	c.PrivateKeyPath = getString(raw, "keys.private_key", c.PrivateKeyPath)
	c.PublicKeyPath = getString(raw, "keys.public_key", c.PublicKeyPath)
	c.OIDCPrivateKeyPath = getString(raw, "oidc.private_key", c.OIDCPrivateKeyPath)
	c.OIDCPublicKeyPath = getString(raw, "oidc.public_key", c.OIDCPublicKeyPath)
	c.IdentityEncryptionKey = getString(raw, "identity.encryption_key", c.IdentityEncryptionKey)
	c.CORSOrigins = getStringSlice(raw, "cors.allow_origins", c.CORSOrigins)
	c.CORSCredentials = getBool(raw, "cors.allow_credentials", c.CORSCredentials)
}
