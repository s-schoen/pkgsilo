package core

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Empty values fall back to the defaults, isolating the test from the shell environment.
	t.Setenv("ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "prod", cfg.Env)
	assert.False(t, cfg.IsDev())
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Empty(t, cfg.TrustedProxies)
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("ENV", "dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/16")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.True(t, cfg.IsDev())
	assert.Equal(t, "127.0.0.1:9000", cfg.HTTPAddr)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, TrustedProxies{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}, cfg.TrustedProxies)
}

func TestLoadConfig_Invalid(t *testing.T) {
	tests := map[string]string{
		"LOG_LEVEL":       "loud",
		"TRUSTED_PROXIES": "10.0.0.1,proxy.local",
	}

	for key, value := range tests {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)

			_, err := LoadConfig()
			assert.Error(t, err)
		})
	}
}
