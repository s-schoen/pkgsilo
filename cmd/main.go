// Command pkgsilo serves the HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"pkgsilo/app"
	"pkgsilo/core/config"
	"pkgsilo/core/database"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/core/server"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		// The process exits either way, so a failed write has nowhere left to be reported.
		_, _ = fmt.Fprintln(os.Stderr, err)
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

	log.Info("starting pkgsilo", "version", version)

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

	h := app.NewHandler(router.Options{TrustedProxies: cfg.TrustedProxies, Dev: cfg.IsDev()}, db, cfg.APIKeyPepper, log)

	if err := server.Run(ctx, server.Options{Addr: cfg.HTTPAddr}, log, h); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
