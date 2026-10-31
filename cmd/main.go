package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/config"
	"pkgsilo/core/database"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/core/server"
	healthapi "pkgsilo/health/api"
	healthrepo "pkgsilo/health/repository"
	healthservice "pkgsilo/health/service"
	identityapi "pkgsilo/identity/api"
	identityrepo "pkgsilo/identity/repository"
	identityservice "pkgsilo/identity/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Restore default signal handling once the first signal arrives, so a second one force-quits.
	context.AfterFunc(ctx, stop)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logger.New(os.Stderr, logger.Options{Dev: cfg.IsDev(), Level: cfg.LogLevel})
	// Only for third-party code logging through slog: our own code uses the injected logger.
	slog.SetDefault(logger)

	db, err := database.Open(ctx, database.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}()

	logger.Info("database connected")

	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db))

	r := router.New(router.Options{TrustedProxies: cfg.TrustedProxies, Dev: cfg.IsDev()}, logger)
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/health", healthapi.NewHandler(health, logger).Routes())
		r.Mount("/identities", identityapi.NewHandler(identities, logger).Routes())
	})

	return server.Run(ctx, server.Options{Addr: cfg.HTTPAddr}, logger, r)
}
