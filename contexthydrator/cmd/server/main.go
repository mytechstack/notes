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
	"github.com/yourorg/context-hydrator/internal/hydrator"
	"github.com/yourorg/context-hydrator/internal/observability"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
	"github.com/yourorg/context-hydrator/internal/services"
)

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

	httpClient := services.NewHTTPClient()
	backend := services.NewBackend(services.BackendConfig{
		ProfileURL:     cfg.ProfileServiceURL,
		PreferencesURL: cfg.PreferencesServiceURL,
		PermissionsURL: cfg.PermissionsServiceURL,
		ResourcesURL:   cfg.ResourcesServiceURL,
	}, httpClient)

	backendTimeout := time.Duration(cfg.BackendTimeoutSecs) * time.Second
	hyd := hydrator.New(store, backend, log, backendTimeout)

	decoder := cookie.NewDecoder(cfg.CookieEncoding, cfg.CookieSecret)

	srv := api.NewServer(store, hyd, decoder, backend, log)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received")

	// 15s covers max 4s backend timeout + Redis writes for in-flight goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", "error", err)
	}
	log.Info("server stopped")
}
