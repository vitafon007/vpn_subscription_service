// Package config загружает конфигурацию сервиса из переменных окружения.
package config

import (
	"os"
	"strconv"
)

// Config содержит настройки приложения, читаемые из окружения.
type Config struct {
	// Port — порт HTTP-сервера.
	Port int
	// DatabaseURL — строка подключения к Postgres.
	DatabaseURL string
	// GinMode — режим Gin (debug, release, test).
	GinMode string
	// BasePath — префикс маршрутов (например /babasub).
	BasePath string
}

// Load читает конфигурацию из переменных окружения и применяет значения по умолчанию.
func Load() Config {
	return Config{
		Port:        envInt("PORT", 23452),
		DatabaseURL: env("DATABASE_URL", "postgres://vpn:vpn@localhost:5432/vpn_sub?sslmode=disable"),
		GinMode:     env("GIN_MODE", "debug"),
		BasePath:    env("BASE_PATH", "/babasub"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
