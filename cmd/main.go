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

	r := core.NewRouter(cfg, logger)
	r.Mount("/health", health.NewHandler().Routes())

	return core.RunServer(ctx, cfg, logger, r)
}
