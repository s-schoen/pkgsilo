package logger

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"

	"pkgsilo/core/config"
	"pkgsilo/core/middleware"
)

// New returns a colored, human-readable logger in development and a JSON logger otherwise.
// Records logged with a request context automatically include its request ID.
func New(w io.Writer, cfg config.Config) *slog.Logger {
	var h slog.Handler
	if cfg.IsDev() {
		h = tint.NewTextHandler(w, &tint.Options{
			Level:      cfg.LogLevel,
			TimeFormat: time.Kitchen,
		})
	} else {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: cfg.LogLevel})
	}

	return slog.New(contextHandler{h})
}

// contextHandler adds request-scoped values from the context to every record.
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := middleware.RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}

	return h.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup must re-wrap the derived handler, otherwise loggers created via
// With or WithGroup would silently fall back to the bare handler and drop the request ID.
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
