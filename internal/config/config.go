package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
}

// Load reads configuration from environment variables.
// Defaults are provided for local development.
func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/recommendations?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
