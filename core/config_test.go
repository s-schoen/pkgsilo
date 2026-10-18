package core

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Empty values fall back to the defaults, isolating the test from the shell environment.
	t.Setenv("ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "prod", cfg.Env)
	assert.False(t, cfg.IsDev())
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("ENV", "dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.True(t, cfg.IsDev())
	assert.Equal(t, "127.0.0.1:9000", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "loud")

	_, err := LoadConfig()
	assert.Error(t, err)
}
