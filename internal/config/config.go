// Package config загружает конфигурацию сервиса из переменных окружения.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
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
	// XUIInsecureSkipVerify — не проверять TLS-сертификат панели (HTTPS на IP).
	XUIInsecureSkipVerify bool
	// PublicBaseURL — публичный базовый URL сервиса для subscription_url.
	PublicBaseURL string

	// InviteGateUID — секретный путь QR-входа (только редирект).
	InviteGateUID string
	// InvitePageUID — секретный путь countdown/открытки.
	InvitePageUID string
	// InviteAdminUID — секретный путь админ-таймлайна.
	InviteAdminUID string
	// InviteTZ — IANA timezone для reveal (по умолчанию Europe/Saratov).
	InviteTZ string
	// InviteRevealAt — локальное время открытия открытки в InviteTZ (RFC3339 без зоны или дата+время).
	InviteRevealAt string
	// InviteVideoPath — путь к mp4 на диске контейнера.
	InviteVideoPath string
}

// Load читает конфигурацию из переменных окружения и применяет значения по умолчанию.
func Load() Config {
	return Config{
		Port:                  envInt("PORT", 23452),
		DatabaseURL:           env("DATABASE_URL", "postgres://vpn:vpn@localhost:5432/vpn_sub?sslmode=disable"),
		GinMode:               env("GIN_MODE", "debug"),
		AdminToken:            env("ADMIN_TOKEN", ""),
		XUIBaseURL:            strings.TrimRight(env("XUI_BASE_URL", ""), "/"),
		XUIAPIToken:           env("XUI_API_TOKEN", ""),
		XUIInsecureSkipVerify: envBool("XUI_INSECURE_SKIP_VERIFY", false),
		PublicBaseURL:         strings.TrimRight(env("PUBLIC_BASE_URL", "http://127.0.0.1:23452"), "/"),
		InviteGateUID:         env("INVITE_GATE_UID", ""),
		InvitePageUID:         env("INVITE_PAGE_UID", ""),
		InviteAdminUID:        env("INVITE_ADMIN_UID", ""),
		InviteTZ:              env("INVITE_TZ", "Europe/Saratov"),
		InviteRevealAt:        env("INVITE_REVEAL_AT", "2026-09-25T20:00:00"),
		InviteVideoPath:       env("INVITE_VIDEO_PATH", "/data/invite/card.mp4"),
	}
}

// XUIConfigured сообщает, заданы ли параметры доступа к 3x-ui.
func (c Config) XUIConfigured() bool {
	return c.XUIBaseURL != "" && c.XUIAPIToken != ""
}

// InviteEnabled — включён ли секретный лендинг (все три UID заданы).
func (c Config) InviteEnabled() bool {
	return c.InviteGateUID != "" && c.InvitePageUID != "" && c.InviteAdminUID != ""
}

// InviteSecureCookie — Secure-флаг cookie при https PUBLIC_BASE_URL.
func (c Config) InviteSecureCookie() bool {
	return strings.HasPrefix(strings.ToLower(c.PublicBaseURL), "https://")
}

// InviteRevealTime парсит момент открытия открытки в заданной TZ.
func (c Config) InviteRevealTime() (time.Time, error) {
	loc, err := time.LoadLocation(c.InviteTZ)
	if err != nil {
		return time.Time{}, err
	}
	raw := strings.TrimSpace(c.InviteRevealAt)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	// Fallback: 25 Sep 2026 20:00 Саратов.
	return time.Date(2026, 9, 25, 20, 0, 0, 0, loc), nil
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

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	// Убираем кавычки из .env вида "true" / 'true'.
	v = strings.Trim(v, `"'`)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		switch v {
		case "yes", "y", "on":
			return true
		case "no", "n", "off":
			return false
		default:
			return fallback
		}
	}
	return b
}
