// Package logger creates the application's structured logger.
package logger

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"

	"pkgsilo/core/middleware"
)

// Options configures the logger.
type Options struct {
	// Dev selects colored, human-readable output instead of JSON.
	Dev   bool
	Level slog.Level
}

// New returns a colored, human-readable logger in development and a JSON logger otherwise.
// Records logged with a request context automatically include its request ID and the attributes
// added with middleware.AddLogAttrs.
func New(w io.Writer, opts Options) *slog.Logger {
	var h slog.Handler
	if opts.Dev {
		h = tint.NewTextHandler(w, &tint.Options{
			Level:      opts.Level,
			TimeFormat: time.Kitchen,
		})
	} else {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: opts.Level})
	}

	return slog.New(contextHandler{h})
}

// contextHandler adds request-scoped values from the context to every record.
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := middleware.RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id)) //nolint:sloglint // The one place that adds the request ID.
	}
	r.AddAttrs(middleware.LogAttrsFromContext(ctx)...)

	return h.Handler.Handle(ctx, r) //nolint:wrapcheck // A decorating handler passes errors through unchanged.
}

// WithAttrs and WithGroup must re-wrap the derived handler, otherwise loggers created via
// With or WithGroup would silently fall back to the bare handler and drop the request ID.
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
