package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("RAILWAY_ENVIRONMENT", "")
	t.Setenv("RAILWAY_PROJECT_ID", "")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("JWT_REFRESH_SECRET", "")
	t.Setenv("PORT", "")
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Fatalf("port: %s", cfg.HTTPPort)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Fatalf("access ttl: %s", cfg.JWTAccessTTL)
	}
	if len(cfg.JWTAccessSecret) < 32 {
		t.Fatal("dev secret should be filled in")
	}
}

func TestPortOverridesHTTPPort(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPPort != "9090" {
		t.Fatalf("expected PORT override, got %s", cfg.HTTPPort)
	}
}

func TestProductionRequiresSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "short")
	t.Setenv("JWT_REFRESH_SECRET", "short")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")

	if _, err := Load(); err == nil {
		t.Fatal("expected production secret validation error")
	}
}

func TestProductionRequiresServiceToken(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "production-access-secret-min-32-chars!!")
	t.Setenv("JWT_REFRESH_SECRET", "production-refresh-secret-min-32-chars!")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing INTERNAL_SERVICE_TOKEN error")
	}
}

func TestRailwayImpliesProduction(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("RAILWAY_ENVIRONMENT", "production")
	t.Setenv("JWT_ACCESS_SECRET", "short")
	t.Setenv("JWT_REFRESH_SECRET", "short")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")

	if _, err := Load(); err == nil {
		t.Fatal("expected Railway deploys to require production secrets")
	}
}

func TestProductionDisablesLocalhostMediaMTX(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "production-access-secret-min-32-chars!!")
	t.Setenv("JWT_REFRESH_SECRET", "production-refresh-secret-min-32-chars!")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "production-internal-token-min-32-chars!")
	t.Setenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable")
	t.Setenv("MEDIAMTX_ENABLED", "true")
	t.Setenv("MEDIAMTX_API_URL", "http://localhost:9997")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.MediaMTX.Enabled {
		t.Fatal("localhost MediaMTX should be disabled in production")
	}
}

func TestRejectsMySQL(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "mysql://user:pass@localhost:3306/cohi")

	if _, err := Load(); err == nil {
		t.Fatal("expected MySQL DATABASE_URL to be rejected")
	}
}
