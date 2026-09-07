package worker

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cohi-hq/cohi-api/internal/config"
)

type Config struct {
	AppEnv      string
	LogLevel    string
	LogFormat   string
	HTTPHost    string
	HTTPPort    string
	APIURL      string
	ServiceToken string
	PollInterval time.Duration
	MediaMTX    config.MediaMTXConfig
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		AppEnv:       getenv("APP_ENV", "development"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
		LogFormat:    getenv("LOG_FORMAT", "json"),
		HTTPHost:     getenv("HTTP_HOST", "0.0.0.0"),
		HTTPPort:     getenv("HTTP_PORT", "8081"),
		APIURL:       strings.TrimRight(getenv("COHI_API_URL", "http://localhost:8080"), "/"),
		ServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		PollInterval: getenvDuration("WORKER_POLL_INTERVAL", 15*time.Second),
		MediaMTX: config.MediaMTXConfig{
			Enabled:     getenvBool("MEDIAMTX_ENABLED", true),
			APIURL:      strings.TrimRight(getenv("MEDIAMTX_API_URL", "http://localhost:9997"), "/"),
			APIUser:     os.Getenv("MEDIAMTX_API_USER"),
			APIPass:     os.Getenv("MEDIAMTX_API_PASS"),
			Timeout:     getenvDuration("MEDIAMTX_TIMEOUT", 8*time.Second),
			PlaybackURL: strings.TrimRight(getenv("MEDIAMTX_PLAYBACK_URL", "http://localhost:9996"), "/"),
		},
	}
	if port := os.Getenv("PORT"); port != "" {
		cfg.HTTPPort = port
	}
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("COHI_API_URL is required")
	}
	if cfg.ServiceToken == "" {
		if strings.EqualFold(cfg.AppEnv, "production") || strings.EqualFold(cfg.AppEnv, "prod") {
			return nil, fmt.Errorf("INTERNAL_SERVICE_TOKEN is required")
		}
		cfg.ServiceToken = "local-dev-internal-token"
	}
	if cfg.PollInterval < 3*time.Second {
		cfg.PollInterval = 3 * time.Second
	}
	return cfg, nil
}

func (c *Config) Addr() string {
	return c.HTTPHost + ":" + c.HTTPPort
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
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
