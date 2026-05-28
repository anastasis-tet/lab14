package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL      = "https://eonet.gsfc.nasa.gov/api/v3"
	defaultHTTPAddr     = ":8080"
	defaultPollInterval = 30 * time.Second
	defaultWindowSize   = 24 * time.Hour
	defaultDays         = 365
	defaultStatus       = "all"
)

type Config struct {
	BaseURL       string
	Categories    []string
	Days          int
	Status        string
	HTTPAddr      string
	PollInterval  time.Duration
	WindowSize    time.Duration
	EtcdEndpoints []string
	NATSURL       string
	NATSSubject   string
}

func Load() (Config, error) {
	cfg := Config{
		BaseURL:       getEnv("EONET_BASE_URL", defaultBaseURL),
		Categories:    splitCSV(getEnv("EONET_CATEGORIES", "wildfires,severeStorms,drought,seaLakeIce,dustHaze")),
		Days:          getEnvInt("EONET_DAYS", defaultDays),
		Status:        getEnv("EONET_STATUS", defaultStatus),
		HTTPAddr:      getEnv("COLLECTOR_HTTP_ADDR", defaultHTTPAddr),
		PollInterval:  getEnvDuration("COLLECTOR_POLL_INTERVAL", defaultPollInterval),
		WindowSize:    getEnvDuration("WINDOW_SIZE", defaultWindowSize),
		EtcdEndpoints: splitCSV(getEnv("ETCD_ENDPOINTS", "")),
		NATSURL:       getEnv("NATS_URL", ""),
		NATSSubject:   getEnv("NATS_SUBJECT", "climate.aggregates"),
	}
	return cfg, cfg.Validate()
}

func (cfg Config) Validate() error {
	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
		return fmt.Errorf("invalid EONET_BASE_URL: %w", err)
	}
	if len(cfg.Categories) == 0 {
		return errors.New("EONET_CATEGORIES must contain at least one category")
	}
	if cfg.Days <= 0 || cfg.Days > 3650 {
		return errors.New("EONET_DAYS must be in range 1..3650")
	}
	if cfg.PollInterval <= 0 {
		return errors.New("COLLECTOR_POLL_INTERVAL must be positive")
	}
	if cfg.WindowSize <= 0 {
		return errors.New("WINDOW_SIZE must be positive")
	}
	return nil
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
