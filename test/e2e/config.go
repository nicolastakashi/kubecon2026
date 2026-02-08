package e2e

import (
	"os"
	"strconv"
	"time"
)

// TestConfig holds configuration for e2e tests
type TestConfig struct {
	BaseURL string
	Timeout time.Duration
	Debug   bool
}

// LoadConfig loads test configuration from environment variables
func LoadConfig() *TestConfig {
	return &TestConfig{
		BaseURL: getEnv("TEST_BASE_URL", "http://localhost:8080"),
		Timeout: getDurationEnv("TEST_TIMEOUT", 30*time.Second),
		Debug:   getBoolEnv("TEST_DEBUG", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
