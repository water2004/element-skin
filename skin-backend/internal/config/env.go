package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func applyEnvOverrides(cfg *Config, raw rawConfig) (bool, error) {
	changed := false
	var applied bool
	var err error
	for _, apply := range []func() (bool, error){
		func() (bool, error) { return applyStringEnv(raw, "JWT_SECRET", "jwt.secret", &cfg.JWTSecret) },
		func() (bool, error) {
			return applyIntEnv(raw, "JWT_EXPIRE_DAYS", "jwt.expire_days", &cfg.JWTExpireDays, positiveInt)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "JWT_ACCESS_EXPIRE_MINUTES", "jwt.access_expire_minutes", &cfg.AccessMinutes, positiveInt)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "KEYS_PRIVATE_KEY", "keys.private_key", &cfg.PrivateKeyPath)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "KEYS_PUBLIC_KEY", "keys.public_key", &cfg.PublicKeyPath)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "OIDC_PRIVATE_KEY", "oidc.private_key", &cfg.OIDCPrivateKeyPath)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "OIDC_PUBLIC_KEY", "oidc.public_key", &cfg.OIDCPublicKeyPath)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "IDENTITY_ENCRYPTION_KEY", "identity.encryption_key", &cfg.IdentityEncryptionKey)
		},
		func() (bool, error) { return applyStringEnv(raw, "DATABASE_HOST", "database.host", &cfg.DatabaseHost) },
		func() (bool, error) { return applyStringEnv(raw, "DATABASE_PORT", "database.port", &cfg.DatabasePort) },
		func() (bool, error) { return applyStringEnv(raw, "DATABASE_USER", "database.user", &cfg.DatabaseUser) },
		func() (bool, error) {
			return applyStringEnv(raw, "DATABASE_PASSWORD", "database.password", &cfg.DatabasePassword)
		},
		func() (bool, error) { return applyStringEnv(raw, "DATABASE_NAME", "database.name", &cfg.DatabaseName) },
		func() (bool, error) {
			return applyStringEnv(raw, "DATABASE_SSLMODE", "database.sslmode", &cfg.DatabaseSSLMode)
		},
		func() (bool, error) {
			return applyInt32Env(raw, "DATABASE_MAX_CONNECTIONS", "database.max_connections", &cfg.MaxConnections, positiveInt)
		},
		func() (bool, error) {
			return applyInt32Env(raw, "WEBHOOK_WORKER_MAX_DATABASE_CONNECTIONS", "webhook_worker.max_database_connections", &cfg.WebhookWorkerMaxConnections, positiveInt)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "WEBHOOK_WORKER_ACTIVE_INTERVAL_MS", "webhook_worker.active_interval_ms", &cfg.WebhookWorkerActiveIntervalMS, positiveInt)
		},
		func() (bool, error) { return applyStringEnv(raw, "SERVER_SITE_URL", "server.site_url", &cfg.SiteURL) },
		func() (bool, error) { return applyStringEnv(raw, "SERVER_API_URL", "server.api_url", &cfg.APIURL) },
		func() (bool, error) { return applyStringEnv(raw, "SERVER_HOST", "server.host", &cfg.ServerHost) },
		func() (bool, error) { return applyServerPortEnv(raw, cfg) },
		func() (bool, error) {
			return applyStringEnv(raw, "TEXTURES_DIRECTORY", "textures.directory", &cfg.TexturesDir)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "CAROUSEL_DIRECTORY", "carousel.directory", &cfg.CarouselDir)
		},
		func() (bool, error) { return applyStringEnv(raw, "REDIS_HOST", "redis.host", &cfg.RedisHost) },
		func() (bool, error) { return applyStringEnv(raw, "REDIS_PORT", "redis.port", &cfg.RedisPort) },
		func() (bool, error) {
			return applyStringEnv(raw, "REDIS_PASSWORD", "redis.password", &cfg.RedisPassword)
		},
		func() (bool, error) { return applyIntEnv(raw, "REDIS_DB", "redis.db", &cfg.RedisDB, nonNegativeInt) },
		func() (bool, error) {
			return applyStringEnv(raw, "REDIS_KEY_PREFIX", "redis.key_prefix", &cfg.RedisKeyPrefix)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "REDIS_PUBLIC_CACHE_TTL_SECONDS", "redis.public_cache_ttl_seconds", &cfg.PublicCacheTTL, positiveInt)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "REDIS_AUTH_CACHE_TTL_SECONDS", "redis.auth_cache_ttl_seconds", &cfg.AuthCacheTTL, positiveInt)
		},
		func() (bool, error) {
			return applyStringSliceEnv(raw, "CORS_ALLOW_ORIGINS", "cors.allow_origins", &cfg.CORSOrigins)
		},
		func() (bool, error) {
			return applyBoolEnv(raw, "CORS_ALLOW_CREDENTIALS", "cors.allow_credentials", &cfg.CORSCredentials)
		},
	} {
		applied, err = apply()
		if err != nil {
			return false, err
		}
		changed = changed || applied
	}
	return changed, nil
}

func applyStringEnv(raw rawConfig, envName, dotted string, target *string) (bool, error) {
	value := os.Getenv(envName)
	if value == "" {
		return false, nil
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func applyIntEnv(raw rawConfig, envName, dotted string, target *int, valid func(int) bool) (bool, error) {
	value, ok, err := parseIntEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if valid != nil && !valid(value) {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func applyInt32Env(raw rawConfig, envName, dotted string, target *int32, valid func(int) bool) (bool, error) {
	value, ok, err := parseIntEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if (valid != nil && !valid(value)) || value > math.MaxInt32 {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = int32(value)
	setRaw(raw, dotted, value)
	return true, nil
}

func applyServerPortEnv(raw rawConfig, cfg *Config) (bool, error) {
	value, ok, err := parseIntEnv("SERVER_PORT")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if !positiveInt(value) {
		return false, fmt.Errorf("invalid environment variable SERVER_PORT")
	}
	cfg.ServerPort = strconv.Itoa(value)
	setRaw(raw, "server.port", value)
	return true, nil
}

func applyStringSliceEnv(raw rawConfig, envName, dotted string, target *[]string) (bool, error) {
	value := os.Getenv(envName)
	if value == "" {
		return false, nil
	}
	items := splitEnvList(value)
	if len(items) == 0 {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = items
	setRaw(raw, dotted, items)
	return true, nil
}

func applyBoolEnv(raw rawConfig, envName, dotted string, target *bool) (bool, error) {
	value, ok, err := parseBoolEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func parseIntEnv(envName string) (int, bool, error) {
	raw := os.Getenv(envName)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("invalid environment variable %s", envName)
	}
	return value, true, nil
}

func parseBoolEnv(envName string) (bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false, fmt.Errorf("invalid environment variable %s", envName)
	}
	return value, true, nil
}

func splitEnvList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func positiveInt(value int) bool {
	return value > 0
}

func nonNegativeInt(value int) bool {
	return value >= 0
}
