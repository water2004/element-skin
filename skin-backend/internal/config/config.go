package config

import (
	"errors"
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DatabaseDSN     string
	MaxConnections  int32
	JWTSecret       string
	JWTExpireDays   int
	AccessMinutes   int
	SiteURL         string
	APIURL          string
	ServerHost      string
	ServerPort      string
	TexturesDir     string
	CarouselDir     string
	FallbackDomains []string
}

type rawConfig = map[string]any

func Load(path string) (Config, error) {
	cfg := Defaults()
	if b, err := os.ReadFile(path); err == nil {
		var raw rawConfig
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return cfg, err
		}
		cfg.apply(raw)
	} else if errors.Is(err, os.ErrNotExist) {
		log.Printf("警告：配置文件 %s 未找到，使用默认配置（JWT secret 为占位值，启动将失败）", path)
	} else {
		return cfg, err
	}
	if env := os.Getenv("DATABASE_DSN"); env != "" {
		cfg.DatabaseDSN = env
	}
	if env := os.Getenv("JWT_SECRET"); env != "" {
		cfg.JWTSecret = env
	}
	return cfg, nil
}

func Defaults() Config {
	return Config{
		DatabaseDSN:    "postgresql://elementskin:password@localhost:5432/elementskin",
		MaxConnections: 10,
		JWTSecret:      "dev-secret-please-change-to-a-very-long-string-in-production",
		JWTExpireDays:  7,
		AccessMinutes:  30,
		SiteURL:        "http://localhost",
		APIURL:         "",
		ServerHost:     "0.0.0.0",
		ServerPort:     "8000",
		TexturesDir:    "textures",
		CarouselDir:    "carousel",
		FallbackDomains: []string{
			"textures.minecraft.net",
		},
	}
}

func (c *Config) apply(raw rawConfig) {
	c.DatabaseDSN = getString(raw, "database.dsn", c.DatabaseDSN)
	if n := getInt(raw, "database.max_connections", int(c.MaxConnections)); n > 0 {
		c.MaxConnections = int32(n)
	}
	c.JWTSecret = getString(raw, "jwt.secret", c.JWTSecret)
	c.JWTExpireDays = getInt(raw, "jwt.expire_days", c.JWTExpireDays)
	c.AccessMinutes = getInt(raw, "jwt.access_expire_minutes", c.AccessMinutes)
	c.SiteURL = getString(raw, "server.site_url", c.SiteURL)
	c.APIURL = getString(raw, "server.api_url", c.APIURL)
	c.ServerHost = getString(raw, "server.host", c.ServerHost)
	c.ServerPort = strconv.Itoa(getInt(raw, "server.port", atoiDefault(c.ServerPort, 8000)))
	c.TexturesDir = getString(raw, "textures.directory", c.TexturesDir)
	c.CarouselDir = getString(raw, "carousel.directory", c.CarouselDir)
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
