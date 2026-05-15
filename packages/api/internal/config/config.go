package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment         string
	HTTPAddr            string
	GRPCAddr            string
	BlockchainRPC       string
	ZKPServiceURL       string
	DatabaseURL         string
	RedisURL            string
	AdminToken          string
	MaxRequestBodyBytes int64
	RateLimitRequests   int
	RateLimitWindow     time.Duration
	AllowedOrigins      []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Environment:         env("UDDI_ENV", "development"),
		HTTPAddr:            env("UDDI_HTTP_ADDR", ":8080"),
		GRPCAddr:            env("UDDI_GRPC_ADDR", ":9090"),
		BlockchainRPC:       env("UDDI_BLOCKCHAIN_RPC", "ws://localhost:9944"),
		ZKPServiceURL:       env("UDDI_ZKP_SERVICE_URL", "http://localhost:3000"),
		DatabaseURL:         env("UDDI_DATABASE_URL", ""),
		RedisURL:            env("UDDI_REDIS_URL", ""),
		AdminToken:          env("UDDI_ADMIN_TOKEN", ""),
		MaxRequestBodyBytes: int64(envInt("UDDI_MAX_REQUEST_BODY_BYTES", 1_048_576)),
		RateLimitRequests:   envInt("UDDI_RATE_LIMIT_REQUESTS", 120),
		RateLimitWindow:     time.Duration(envInt("UDDI_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
		AllowedOrigins:      splitCSV(env("UDDI_ALLOWED_ORIGINS", "*")),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production")
}

func (c *Config) validate() error {
	if !c.IsProduction() {
		return nil
	}
	if c.AdminToken == "" || c.AdminToken == "dev-admin-token-change-in-production" {
		return errors.New("UDDI_ADMIN_TOKEN must be set to a non-development value in production")
	}
	for _, origin := range c.AllowedOrigins {
		if origin == "*" {
			return errors.New("UDDI_ALLOWED_ORIGINS must not include * in production")
		}
	}
	return nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
