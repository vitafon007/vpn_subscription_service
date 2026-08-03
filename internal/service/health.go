// Package service содержит бизнес-логику без привязки к Gin/HTTP.
package service

import (
	"context"

	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/model"
)

// HealthService реализует liveness и readiness проверки.
type HealthService struct {
	healthDAO *dao.HealthDAO
}

// NewHealthService создаёт HealthService с зависимостью на HealthDAO.
func NewHealthService(healthDAO *dao.HealthDAO) *HealthService {
	return &HealthService{healthDAO: healthDAO}
}

// Liveness возвращает статус «живой» без обращения к БД.
func (s *HealthService) Liveness() model.HealthResponse {
	return model.HealthResponse{Status: "ok"}
}

// Readiness проверяет готовность сервиса, включая доступность Postgres.
func (s *HealthService) Readiness(ctx context.Context) (model.ReadyResponse, error) {
	if err := s.healthDAO.Ping(ctx); err != nil {
		return model.ReadyResponse{}, err
	}
	return model.ReadyResponse{Status: "ready"}, nil
}
