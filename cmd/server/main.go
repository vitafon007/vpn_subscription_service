// Package main запускает HTTP-сервер VPN subscription service.
//
// @title           VPN Subscription Service API
// @version         1.0
// @description     Сервис выдачи VPN-подписок (v1 scaffold: health/ready).
// @host            chistotel.webtm.ru
// @BasePath        /babasub
// @schemes         https
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("подключение к БД: %v", err)
	}
	defer db.Close(pool)

	router := server.NewRouter(cfg, pool)

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("сервер слушает %s (base path %s)", addr, cfg.BasePath)
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
