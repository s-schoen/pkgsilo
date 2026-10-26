package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logger.New(os.Stderr, cfg)
	slog.SetDefault(logger)

	db, err := database.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}()

	logger.Info("database connected")

	r := router.New(cfg, logger)
	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	r.Mount("/health", healthapi.NewHandler(health, logger).Routes())

	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db))
	r.Mount("/identities", identityapi.NewHandler(identities, logger).Routes())

	return server.Run(ctx, cfg, logger, r)
}
