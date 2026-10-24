package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"pkgsilo/core"
	"pkgsilo/health"
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

	cfg, err := core.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := core.NewLogger(os.Stderr, cfg)
	slog.SetDefault(logger)

	db, err := core.OpenDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}()

	logger.Info("database connected")

	r := core.NewRouter(cfg, logger)
	r.Mount("/health", health.NewHandler().Routes())

	return core.RunServer(ctx, cfg, logger, r)
}
