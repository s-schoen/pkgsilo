package config

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/middleware"
)

func TestLoad_Defaults(t *testing.T) {
	// Empty values fall back to the defaults, isolating the test from the shell environment.
	t.Setenv("ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("DATABASE_MAX_CONNS", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "prod", cfg.Env)
	assert.False(t, cfg.IsDev())
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Empty(t, cfg.TrustedProxies)
	assert.Equal(t, "postgres://localhost/pkgsilo", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.DatabaseMaxConns)
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("ENV", "dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/16")
	t.Setenv("DATABASE_URL", "postgres://db.internal/pkgsilo")
	t.Setenv("DATABASE_MAX_CONNS", "25")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.IsDev())
	assert.Equal(t, "127.0.0.1:9000", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, middleware.TrustedProxies{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}, cfg.TrustedProxies)
	assert.Equal(t, "postgres://db.internal/pkgsilo", cfg.DatabaseURL)
	assert.Equal(t, 25, cfg.DatabaseMaxConns)
}

func TestLoad_Invalid(t *testing.T) {
	tests := map[string]string{
		"LOG_LEVEL":          "loud",
		"TRUSTED_PROXIES":    "10.0.0.1,proxy.local",
		"DATABASE_URL":       "",
		"DATABASE_MAX_CONNS": "many",
	}

	for key, value := range tests {
		t.Run(key, func(t *testing.T) {
			// A valid required value, so each case fails only because of its own key.
			t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")
			t.Setenv(key, value)

			_, err := Load()
			assert.Error(t, err)
		})
	}
}
