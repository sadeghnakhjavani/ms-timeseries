package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	CHHost              string
	CHPort              int
	CHUser              string
	CHPassword          string
	CHDatabase          string
	APIKey              string
	AppPort             int
	PageDefaultLimit int
	PageMaxLimit     int
}

func Load() (Config, error) {
	cfg := Config{
		CHHost:     envOrDefault("CH_HOST", "localhost"),
		CHUser:     envOrDefault("CH_USER", "default"),
		CHPassword: os.Getenv("CH_PASSWORD"),
		CHDatabase: envOrDefault("CH_DATABASE", "default"),
	}

	portStr := envOrDefault("CH_PORT", "9000")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid CH_PORT: %w", err)
	}
	cfg.CHPort = port

	appPortStr := envOrDefault("APP_PORT", "8080")
	appPort, err := strconv.Atoi(appPortStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_PORT: %w", err)
	}
	cfg.AppPort = appPort

	cfg.APIKey = os.Getenv("API_KEY")
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("API_KEY is required")
	}

	defaultLimit, err := parsePositiveIntEnv("PAGE_DEFAULT_LIMIT", 1000)
	if err != nil {
		return Config{}, err
	}
	maxLimit, err := parsePositiveIntEnv("PAGE_MAX_LIMIT", 1000)
	if err != nil {
		return Config{}, err
	}
	if defaultLimit > maxLimit {
		return Config{}, fmt.Errorf("PAGE_DEFAULT_LIMIT must not exceed PAGE_MAX_LIMIT")
	}
	cfg.PageDefaultLimit = defaultLimit
	cfg.PageMaxLimit = maxLimit

	return cfg, nil
}

func parsePositiveIntEnv(key string, fallback int) (int, error) {
	raw := envOrDefault(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid %s: must be a positive integer", key)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
