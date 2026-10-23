package core

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// newJSONLogger returns a production (JSON) logger writing into the returned buffer.
func newJSONLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return NewLogger(&buf, Config{Env: "prod", LogLevel: slog.LevelInfo}), &buf
}

// logEntries decodes one JSON object per logged line.
func logEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var entries []map[string]any
	for line := range strings.Lines(buf.String()) {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		entries = append(entries, entry)
	}

	return entries
}
