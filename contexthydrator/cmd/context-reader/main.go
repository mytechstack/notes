package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/context-hydrator/internal/api"
	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/config"
	"github.com/yourorg/context-hydrator/internal/cookie"
	"github.com/yourorg/context-hydrator/internal/observability"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
)

// cmd/context-reader runs the context reader service only (GET /data, GET /context).
// This is the authenticated post-auth service — reads from Redis only.
// For local development use cmd/server (combined).
func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		os.Exit(1)
	}

	log := observability.NewLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	redisClient, err := redisc.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	log.Info("redis connected", "addr", cfg.RedisAddr)

	store := cache.NewStore(redisClient)
	appConfig := cfg.DefaultAppConfig()

	// Context reader has no backend dependency — Redis only.
	// decoder is still needed if you add auth middleware later.
	decoder := cookie.NewDecoder(cfg.CookieEncoding, cfg.CookieSecret)

	srv := api.NewServer(store, nil, decoder, appConfig, log)

	httpServer := &http.Server{
		Addr:         ":" + cfg.ReaderPort,
		Handler:      srv.ReaderHandler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("context reader starting", "port", cfg.ReaderPort, "app_id", appConfig.AppID)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", "error", err)
	}
	log.Info("context reader stopped")
}
