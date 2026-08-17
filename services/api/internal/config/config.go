package config

import (
	"fmt"
	"os"
	"strconv"
)

const defaultMaxUploadBytes int64 = 1024 * 1024 * 1024 // 1 GiB for the local prototype.

type Config struct {
	HTTPAddress      string
	DatabaseURL      string
	VideoStoragePath string
	WebOrigin        string
	MaxUploadBytes   int64
	UploadRateLimit  RateLimit
}

// RateLimit describes a token bucket applied per client.
type RateLimit struct {
	// Burst is the number of requests a single client may make back to back.
	Burst int
	// RefillPerMinute is how many tokens a client regains each minute.
	RefillPerMinute int
}

func Load() (Config, error) {
	maxUploadBytes, err := envInt64OrDefault("MAX_UPLOAD_BYTES", defaultMaxUploadBytes)
	if err != nil {
		return Config{}, err
	}
	if maxUploadBytes <= 0 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be positive, got %d", maxUploadBytes)
	}

	burst, err := envInt64OrDefault("UPLOAD_RATE_LIMIT_BURST", 5)
	if err != nil {
		return Config{}, err
	}
	refill, err := envInt64OrDefault("UPLOAD_RATE_LIMIT_PER_MINUTE", 10)
	if err != nil {
		return Config{}, err
	}
	if burst <= 0 || refill <= 0 {
		return Config{}, fmt.Errorf("upload rate limit values must be positive, got burst %d and refill %d", burst, refill)
	}

	return Config{
		HTTPAddress:      envOrDefault("HTTP_ADDRESS", ":8080"),
		DatabaseURL:      envOrDefault("DATABASE_URL", "postgres://rivo:rivo_dev@localhost:5432/rivo?sslmode=disable"),
		VideoStoragePath: envOrDefault("VIDEO_STORAGE_PATH", "../../data/videos"),
		WebOrigin:        envOrDefault("WEB_ORIGIN", "http://localhost:3000"),
		MaxUploadBytes:   maxUploadBytes,
		UploadRateLimit: RateLimit{
			Burst:           int(burst),
			RefillPerMinute: int(refill),
		},
	}, nil
}

func envOrDefault(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}

func envInt64OrDefault(name string, fallback int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}

	return parsed, nil
}
