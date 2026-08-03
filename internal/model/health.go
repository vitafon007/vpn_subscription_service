// Package model содержит DTO ответов HTTP API.
package model

// HealthResponse — ответ liveness-проверки.
type HealthResponse struct {
	// Status — статус сервиса (ok).
	Status string `json:"status" example:"ok"`
}

// ReadyResponse — ответ readiness-проверки.
type ReadyResponse struct {
	// Status — статус готовности (ready).
	Status string `json:"status" example:"ready"`
}

// ErrorResponse — стандартный ответ об ошибке.
type ErrorResponse struct {
	// Error — текст ошибки.
	Error string `json:"error" example:"database unavailable"`
}
