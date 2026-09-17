// Package server собирает HTTP-роутер и связывает слои приложения.
package server

import (
	"log"
	"net/http"

	"github.com/chistotel/vpn_subscription_service/internal/config"
	"github.com/chistotel/vpn_subscription_service/internal/dao"
	"github.com/chistotel/vpn_subscription_service/internal/handler"
	"github.com/chistotel/vpn_subscription_service/internal/middleware"
	"github.com/chistotel/vpn_subscription_service/internal/service"
	"github.com/chistotel/vpn_subscription_service/internal/xui"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/chistotel/vpn_subscription_service/docs"
)

// NewRouter создаёт Gin-роутер, монтирует API и Swagger на корне.
func NewRouter(cfg config.Config, pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(cfg.GinMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Доверяем локальному nginx на том же хосте.
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	healthDAO := dao.NewHealthDAO(pool)
	healthService := service.NewHealthService(healthDAO)
	healthHandler := handler.NewHealthHandler(healthService)

	userDAO := dao.NewUserDAO(pool)
	titleDAO := dao.NewTitleDAO(pool)
	announceDAO := dao.NewAnnounceDAO(pool)
	xuiClient := xui.New(cfg.XUIBaseURL, cfg.XUIAPIToken, cfg.XUIInsecureSkipVerify)

	adminService := service.NewAdminUserService(userDAO, titleDAO, announceDAO, xuiClient, cfg.PublicBaseURL)
	subService := service.NewSubscriptionService(userDAO, titleDAO, announceDAO, xuiClient)

	adminHandler := handler.NewAdminHandler(adminService)
	subHandler := handler.NewSubscriptionHandler(subService)

	api := r.Group("/api/v1")
	{
		api.GET("/health", healthHandler.Liveness)
		api.GET("/ready", healthHandler.Readiness)
		api.GET("/sub/:token", subHandler.GetSubscription)

		admin := api.Group("/admin")
		admin.Use(middleware.RequireAdmin(cfg.AdminToken))
		{
			admin.POST("/users", adminHandler.BindUser)
			admin.PUT("/users/:login/title", adminHandler.SetTitle)
			admin.POST("/users/:login/announces", adminHandler.AddAnnounce)
			admin.GET("/users/:login/announces", adminHandler.ListAnnounces)
			admin.DELETE("/users/:login/announces/:id", adminHandler.DeleteAnnounce)
		}
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if cfg.InviteEnabled() {
		inviteDAO := dao.NewInviteDAO(pool)
		inviteSvc := service.NewInviteService(inviteDAO, cfg)
		inviteHandler, err := handler.NewInviteHandler(inviteSvc)
		if err != nil {
			log.Fatalf("invite templates: %v", err)
		}

		r.StaticFS("/invite/static", inviteHandler.StaticFS())
		r.POST("/invite/api/events", inviteHandler.PostEvent)
		r.POST("/invite/api/rsvp", inviteHandler.PostRSVP)
		r.GET("/invite/api/calendar.ics", inviteHandler.CalendarICS)

		r.GET("/"+cfg.InviteGateUID, inviteHandler.Gate)
		r.GET("/"+cfg.InvitePageUID, inviteHandler.Page)
		r.GET("/"+cfg.InviteAdminUID, inviteHandler.AdminPage)
		r.GET("/"+cfg.InvitePageUID+"/media/card.mp4", inviteHandler.ServeVideo)
	} else {
		// Заглушки, чтобы случайные /invite/* не светили структуру, если выключено.
		r.Any("/invite/*path", func(c *gin.Context) {
			c.Status(http.StatusNotFound)
		})
	}

	return r
}
