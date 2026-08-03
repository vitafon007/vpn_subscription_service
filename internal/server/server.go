// Package server собирает HTTP-роутер и связывая слои приложения.
package server

import (
	"github.com/chistotel/vpn_subscription_service/internal/config"
	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/handler"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/chistotel/vpn_subscription_service/docs"
)

// NewRouter создаёт Gin-роутер, монтирует API и Swagger под BASE_PATH.
func NewRouter(cfg config.Config, pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(cfg.GinMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Доверяем локальному nginx на том же хосте.
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	healthDAO := dao.NewHealthDAO(pool)
	healthService := service.NewHealthService(healthDAO)
	healthHandler := handler.NewHealthHandler(healthService)

	base := r.Group(cfg.BasePath)
	{
		api := base.Group("/api/v1")
		{
			api.GET("/health", healthHandler.Liveness)
			api.GET("/ready", healthHandler.Readiness)
		}

		base.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return r
}
