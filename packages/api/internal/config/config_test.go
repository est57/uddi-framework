package config

import (
	"strings"
	"testing"
)

func TestLoadDefaultsToDevelopment(t *testing.T) {
	t.Setenv("UDDI_ENV", "")
	t.Setenv("UDDI_ADMIN_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Environment != "development" {
		t.Fatalf("expected development environment, got %s", cfg.Environment)
	}
	if !contains(cfg.AllowedOrigins, "*") {
		t.Fatalf("expected wildcard origin in development defaults")
	}
}

func TestLoadRejectsProductionWithoutSecureAdminToken(t *testing.T) {
	t.Setenv("UDDI_ENV", "production")
	t.Setenv("UDDI_ALLOWED_ORIGINS", "https://app.example")
	t.Setenv("UDDI_ADMIN_TOKEN", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "UDDI_ADMIN_TOKEN") {
		t.Fatalf("expected admin token validation error, got %v", err)
	}
}

func TestLoadRejectsProductionWildcardOrigins(t *testing.T) {
	t.Setenv("UDDI_ENV", "production")
	t.Setenv("UDDI_ALLOWED_ORIGINS", "*")
	t.Setenv("UDDI_ADMIN_TOKEN", "production-admin-token")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "UDDI_ALLOWED_ORIGINS") {
		t.Fatalf("expected allowed origins validation error, got %v", err)
	}
}

func TestLoadAcceptsProductionWithExplicitSecurityConfig(t *testing.T) {
	t.Setenv("UDDI_ENV", "production")
	t.Setenv("UDDI_ALLOWED_ORIGINS", "https://app.example")
	t.Setenv("UDDI_ADMIN_TOKEN", "production-admin-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatalf("expected production config")
	}
	if cfg.AdminToken != "production-admin-token" {
		t.Fatalf("unexpected admin token")
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
