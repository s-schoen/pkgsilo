// Package loggertest provides helpers for asserting on log output in tests.
package loggertest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pkgsilo/core/config"
	"pkgsilo/core/logger"
)

// NewJSON returns a production (JSON) logger writing into the returned buffer.
func NewJSON() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return logger.New(&buf, config.Config{Env: "prod", LogLevel: slog.LevelInfo}), &buf
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
