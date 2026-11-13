// Package loggertest provides loggers for tests and helpers for asserting on their output.
// It must import nothing from pkgsilo but core/logger, so that internal tests of core packages
// such as core/router can use it without an import cycle.
package loggertest

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pkgsilo/core/logger"
)

// Discard returns a logger that drops every record, for tests that don't check the log.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// NewJSON returns a production (JSON) logger writing into the returned buffer.
func NewJSON() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return logger.New(&buf, logger.Options{Level: slog.LevelInfo}), &buf
}

// Entries decodes one JSON object per logged line.
func Entries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var entries []map[string]any
	for line := range strings.Lines(buf.String()) {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		entries = append(entries, entry)
	}

	return entries
}
