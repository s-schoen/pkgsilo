// Command pkgsilo serves the HTTP API.
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

	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"

	"pkgsilo/app"
	"pkgsilo/core/config"
	"pkgsilo/core/database"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/core/server"
	"pkgsilo/core/telemetry"
)

// telemetryShutdownTimeout bounds the shutdown of the telemetry once the servers have stopped.
const telemetryShutdownTimeout = 5 * time.Second

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

	tel, err := telemetry.New(ctx, telemetry.Options{Enabled: cfg.MetricsEnabled(), ServiceVersion: version})
	if err != nil {
		return fmt.Errorf("set up telemetry: %w", err)
	}
	// Only for third-party code recording metrics: our own code uses the injected provider.
	otel.SetMeterProvider(tel.MeterProvider())

	db, err := database.Open(ctx, database.Options{
		URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns, MeterProvider: tel.MeterProvider(),
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("close database", "error", err)
		}
	}()

	log.Info("database connected")

	apiKey, err := app.Bootstrap(ctx, db, cfg.APIKeyPepper)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if apiKey != "" {
		log.Warn("created identity admin with an api key that never expires: create your own key and delete this one",
			"api_key", apiKey)
	}

	h := app.NewHandler(router.Options{TrustedProxies: cfg.TrustedProxies, Dev: cfg.IsDev()}, db, cfg.APIKeyPepper, log,
		tel.MeterProvider())

	return serve(ctx, cfg, log, h, tel)
}

// serve serves the API handler h and, if enabled, the metrics of tel, each on its own listener, until
// ctx is canceled or one of them fails, which stops the other. Then it shuts tel down.
func serve(ctx context.Context, cfg config.Config, log *slog.Logger, h http.Handler, tel *telemetry.Telemetry) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := server.Run(gctx, server.Options{Addr: cfg.HTTPAddr}, log.With("server", "api"), h); err != nil {
			return fmt.Errorf("run api server: %w", err)
		}
		return nil
	})
	if cfg.MetricsEnabled() {
		g.Go(func() error {
			err := server.Run(gctx, server.Options{Addr: cfg.MetricsAddr}, log.With("server", "metrics"), tel.Handler())
			if err != nil {
				return fmt.Errorf("run metrics server: %w", err)
			}
			return nil
		})
	}
	err := g.Wait()

	// Both servers have stopped, so nothing records anymore. ctx may be canceled already.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telemetryShutdownTimeout)
	defer cancel()
	if shutdownErr := tel.Shutdown(shutdownCtx); shutdownErr != nil {
		log.ErrorContext(shutdownCtx, "shut down telemetry", "error", shutdownErr)
	}

	return err //nolint:wrapcheck // Wrapped by the goroutines above.
}
