package core

import (
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"
)

// NewLogger returns a colored, human-readable logger in development and a JSON logger otherwise.
func NewLogger(w io.Writer, cfg Config) *slog.Logger {
	if cfg.IsDev() {
		return slog.New(tint.NewTextHandler(w, &tint.Options{
			Level:      cfg.LogLevel,
			TimeFormat: time.Kitchen,
		}))
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: cfg.LogLevel}))
}
