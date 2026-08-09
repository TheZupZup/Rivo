package config

import "os"

type Config struct {
	HTTPAddress      string
	DatabaseURL      string
	VideoStoragePath string
}

func Load() Config {
	return Config{
		HTTPAddress:      envOrDefault("HTTP_ADDRESS", ":8080"),
		DatabaseURL:      envOrDefault("DATABASE_URL", "postgres://rivo:rivo_dev@localhost:5432/rivo?sslmode=disable"),
		VideoStoragePath: envOrDefault("VIDEO_STORAGE_PATH", "../../data/videos"),
	}
}

func envOrDefault(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}
