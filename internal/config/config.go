package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds process configuration loaded from the environment.
type Config struct {
	AppName string
	AppEnv  string

	HTTPHost string
	HTTPPort string

	TrustedProxies     []string
	CORSAllowedOrigins []string

	LogLevel  string
	LogFormat string

	DatabaseURL      string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	AutoMigrate      bool
	MigrationsPath   string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	JWTIssuer        string

	MediaMTX MediaMTXConfig
}

// MediaMTXConfig controls how cohi-api talks to the media server.
type MediaMTXConfig struct {
	Enabled    bool
	APIURL     string
	APIUser    string
	APIPass    string
	Timeout    time.Duration
	RTSPURL    string
	HLSURL     string
	WebRTCURL  string
}

// Load reads configuration from environment variables and applies defaults.
func Load() (*Config, error) {
	cfg := &Config{
		AppName: getenv("APP_NAME", "cohi-api"),
		AppEnv:  getenv("APP_ENV", "development"),

		HTTPHost: getenv("HTTP_HOST", "0.0.0.0"),
		HTTPPort: getenv("HTTP_PORT", "8080"),

		TrustedProxies:     splitCSV(os.Getenv("TRUSTED_PROXIES")),
		CORSAllowedOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "*")),

		LogLevel:  getenv("LOG_LEVEL", "info"),
		LogFormat: getenv("LOG_FORMAT", defaultLogFormat(getenv("APP_ENV", "development"))),

		DatabaseURL:     getenv("DATABASE_URL", "postgres://cohi:cohi@localhost:5432/cohi?sslmode=disable"),
		MaxOpenConns:    getenvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getenvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getenvDuration("DB_CONN_MAX_LIFETIME", time.Hour),
		AutoMigrate:     getenvBool("AUTO_MIGRATE", true),
		MigrationsPath:  getenv("MIGRATIONS_PATH", "./migrations"),

		JWTAccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		JWTAccessTTL:     getenvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:    getenvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		JWTIssuer:        getenv("JWT_ISSUER", "cohi-api"),

		MediaMTX: MediaMTXConfig{
			Enabled:   getenvBool("MEDIAMTX_ENABLED", true),
			APIURL:    strings.TrimRight(getenv("MEDIAMTX_API_URL", "http://localhost:9997"), "/"),
			APIUser:   os.Getenv("MEDIAMTX_API_USER"),
			APIPass:   os.Getenv("MEDIAMTX_API_PASS"),
			Timeout:   getenvDuration("MEDIAMTX_TIMEOUT", 8*time.Second),
			RTSPURL:   strings.TrimRight(getenv("MEDIAMTX_RTSP_URL", "rtsp://localhost:8554"), "/"),
			HLSURL:    strings.TrimRight(getenv("MEDIAMTX_HLS_URL", "http://localhost:8888"), "/"),
			WebRTCURL: strings.TrimRight(getenv("MEDIAMTX_WEBRTC_URL", "http://localhost:8889"), "/"),
		},
	}

	// Cloud platforms (Railway, Fly, Render, etc.) inject PORT.
	if port := os.Getenv("PORT"); port != "" {
		cfg.HTTPPort = port
	}

	if err := cfg.normalizeAndValidate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) normalizeAndValidate() error {
	if c.HTTPPort == "" {
		return fmt.Errorf("HTTP_PORT (or PORT) is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTAccessTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL must be positive")
	}
	if c.JWTRefreshTTL <= 0 {
		return fmt.Errorf("JWT_REFRESH_TTL must be positive")
	}
	if c.JWTRefreshTTL <= c.JWTAccessTTL {
		return fmt.Errorf("JWT_REFRESH_TTL must be greater than JWT_ACCESS_TTL")
	}

	if c.IsProduction() {
		if len(c.JWTAccessSecret) < 32 {
			return fmt.Errorf("JWT_ACCESS_SECRET must be at least 32 characters in production")
		}
		if len(c.JWTRefreshSecret) < 32 {
			return fmt.Errorf("JWT_REFRESH_SECRET must be at least 32 characters in production")
		}
	} else {
		if c.JWTAccessSecret == "" {
			c.JWTAccessSecret = "dev-only-access-secret-change-me-32b"
		}
		if c.JWTRefreshSecret == "" {
			c.JWTRefreshSecret = "dev-only-refresh-secret-change-me-32"
		}
	}

	if c.MediaMTX.Enabled && c.MediaMTX.APIURL == "" {
		return fmt.Errorf("MEDIAMTX_API_URL is required when MediaMTX is enabled")
	}

	return nil
}

// Addr returns the HTTP bind address.
func (c *Config) Addr() string {
	return c.HTTPHost + ":" + c.HTTPPort
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production") || strings.EqualFold(c.AppEnv, "prod")
}

func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(c.AppEnv, "development") || strings.EqualFold(c.AppEnv, "dev")
}

func defaultLogFormat(env string) string {
	if strings.EqualFold(env, "development") || strings.EqualFold(env, "dev") {
		return "text"
	}
	return "json"
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
