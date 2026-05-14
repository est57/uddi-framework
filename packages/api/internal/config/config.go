package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	BlockchainRPC  string
	ZKPServiceURL  string
	DatabaseURL    string
	RedisURL       string
	AdminToken     string
	AllowedOrigins []string
}

func Load() (*Config, error) {
	return &Config{
		HTTPAddr:       env("UDDI_HTTP_ADDR", ":8080"),
		GRPCAddr:       env("UDDI_GRPC_ADDR", ":9090"),
		BlockchainRPC:  env("UDDI_BLOCKCHAIN_RPC", "ws://localhost:9944"),
		ZKPServiceURL:  env("UDDI_ZKP_SERVICE_URL", "http://localhost:3000"),
		DatabaseURL:    env("UDDI_DATABASE_URL", ""),
		RedisURL:       env("UDDI_REDIS_URL", ""),
		AdminToken:     env("UDDI_ADMIN_TOKEN", ""),
		AllowedOrigins: splitCSV(env("UDDI_ALLOWED_ORIGINS", "*")),
	}, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
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
