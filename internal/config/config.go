// Package config загружает конфигурацию сервиса из переменных окружения.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config содержит настройки приложения, читаемые из окружения.
type Config struct {
	// Port — порт HTTP-сервера.
	Port int
	// DatabaseURL — строка подключения к Postgres.
	DatabaseURL string
	// GinMode — режим Gin (debug, release, test).
	GinMode string
	// AdminToken — Bearer-токен для админ-API; пустой = deny-all.
	AdminToken string
	// XUIBaseURL — базовый URL панели 3x-ui (без завершающего слэша).
	XUIBaseURL string
	// XUIAPIToken — Bearer-токен API 3x-ui.
	XUIAPIToken string
	// PublicBaseURL — публичный базовый URL сервиса для subscription_url.
	PublicBaseURL string
}

// Load читает конфигурацию из переменных окружения и применяет значения по умолчанию.
func Load() Config {
	return Config{
		Port:          envInt("PORT", 23452),
		DatabaseURL:   env("DATABASE_URL", "postgres://vpn:vpn@localhost:5432/vpn_sub?sslmode=disable"),
		GinMode:       env("GIN_MODE", "debug"),
		AdminToken:    env("ADMIN_TOKEN", ""),
		XUIBaseURL:    strings.TrimRight(env("XUI_BASE_URL", ""), "/"),
		XUIAPIToken:   env("XUI_API_TOKEN", ""),
		PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", "http://127.0.0.1:23452"), "/"),
	}
}

// XUIConfigured сообщает, заданы ли параметры доступа к 3x-ui.
func (c Config) XUIConfigured() bool {
	return c.XUIBaseURL != "" && c.XUIAPIToken != ""
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
