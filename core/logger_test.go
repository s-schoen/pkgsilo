package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_AddsRequestIDFromContext(t *testing.T) {
	logger, buf := newJSONLogger()
	ctx := context.WithValue(t.Context(), requestIDKey{}, "req-1")

	logger.InfoContext(ctx, "plain")
	logger.With("component", "test").InfoContext(ctx, "derived via With")
	logger.WithGroup("grp").InfoContext(ctx, "derived via WithGroup")
	logger.InfoContext(t.Context(), "no request")

	entries := logEntries(t, buf)
	require.Len(t, entries, 4)

	assert.Equal(t, "req-1", entries[0]["request_id"])
	assert.Equal(t, "req-1", entries[1]["request_id"])
	assert.Equal(t, map[string]any{"request_id": "req-1"}, entries[2]["grp"])
	assert.NotContains(t, entries[3], "request_id")
}
