// Package handler содержит HTTP-обработчики Gin без прямого доступа к БД.
package handler

import (
	"net/http"

	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/gin-gonic/gin"
)

// HealthHandler обрабатывает health/ready эндпоинты.
type HealthHandler struct {
	healthService *service.HealthService
}

// NewHealthHandler создаёт HealthHandler с зависимостью на HealthService.
func NewHealthHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{healthService: healthService}
}

// Liveness godoc
// @Summary      Liveness probe
// @Description  Проверка, что процесс жив (без обращения к БД)
// @Tags         health
// @Produce      json
// @Success      200  {object}  model.HealthResponse
// @Router       /api/v1/health [get]
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, h.healthService.Liveness())
}

// Readiness godoc
// @Summary      Readiness probe
// @Description  Проверка готовности сервиса, включая Postgres
// @Tags         health
// @Produce      json
// @Success      200  {object}  model.ReadyResponse
// @Failure      503  {object}  model.ErrorResponse
// @Router       /api/v1/ready [get]
func (h *HealthHandler) Readiness(c *gin.Context) {
	resp, err := h.healthService.Readiness(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Error: "database unavailable"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
