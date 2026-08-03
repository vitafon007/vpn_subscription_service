// Package main запускает HTTP-сервер VPN subscription service.
//
// @title           VPN Subscription Service API
// @version         1.0
// @description     Сервис выдачи VPN-подписок с интеграцией 3x-ui (bind + title/announce overlay).
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer ADMIN_TOKEN
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chistotel/vpn_subscription_service/internal/config"
	"github.com/chistotel/vpn_subscription_service/internal/db"
	"github.com/chistotel/vpn_subscription_service/internal/server"
)

func main() {
	cfg := config.Load()

	if cfg.AdminToken == "" {
		log.Printf("предупреждение: ADMIN_TOKEN пуст — админ-API отклонит все запросы")
	}
	if !cfg.XUIConfigured() {
		log.Printf("предупреждение: XUI_BASE_URL/XUI_API_TOKEN не заданы — bind/sub вернут 503")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("подключение к БД: %v", err)
	}
	defer db.Close(pool)

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("миграции: %v", err)
	}

	router := server.NewRouter(cfg, pool)

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("сервер слушает %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ошибка сервера: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}
