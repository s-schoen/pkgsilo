// Command pkgsilo serves the HTTP API.
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

	log := logger.New(os.Stderr, logger.Options{Dev: cfg.IsDev(), Level: cfg.LogLevel})
	// Only for third-party code logging through slog: our own code uses the injected logger.
	slog.SetDefault(log)

	db, err := database.Open(ctx, database.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("close database", "error", err)
		}
	}()

	log.Info("database connected")

	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db))

	r := router.New(router.Options{TrustedProxies: cfg.TrustedProxies, Dev: cfg.IsDev()}, log)
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/health", healthapi.NewHandler(health, log).Routes())
		r.Mount("/identities", identityapi.NewHandler(identities, log).Routes())
	})

	if err := server.Run(ctx, server.Options{Addr: cfg.HTTPAddr}, log, r); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
