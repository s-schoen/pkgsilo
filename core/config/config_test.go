package config

import (
	"log/slog"
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPepper = "test-pepper-of-at-least-32-characters"

// unsetenv unsets the environment variable key for the test and restores it afterwards.
func unsetenv(t *testing.T, key string) {
	t.Helper()

	// Registers the restore, which os.Unsetenv alone wouldn't.
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

func TestLoad_Defaults(t *testing.T) {
	// Empty values fall back to the defaults, isolating the test from the shell environment.
	t.Setenv("ENV", "")
	t.Setenv("HTTP_ADDR", "")
	unsetenv(t, "METRICS_ADDR")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("DATABASE_MAX_CONNS", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")
	t.Setenv("API_KEY_PEPPER", testPepper)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "prod", cfg.Env)
	assert.False(t, cfg.IsDev())
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, ":9090", cfg.MetricsAddr)
	assert.True(t, cfg.MetricsEnabled())
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Empty(t, cfg.TrustedProxies)
	assert.Equal(t, "postgres://localhost/pkgsilo", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.DatabaseMaxConns)
	assert.False(t, cfg.TracingEnabled())
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("ENV", "dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("METRICS_ADDR", "127.0.0.1:9100")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/16")
	t.Setenv("DATABASE_URL", "postgres://db.internal/pkgsilo")
	t.Setenv("DATABASE_MAX_CONNS", "25")
	t.Setenv("API_KEY_PEPPER", testPepper)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://collector:4318/v1/traces")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.IsDev())
	assert.Equal(t, "127.0.0.1:9000", cfg.HTTPAddr)
	assert.Equal(t, "127.0.0.1:9100", cfg.MetricsAddr)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, Prefixes{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}, cfg.TrustedProxies)
	assert.Equal(t, "postgres://db.internal/pkgsilo", cfg.DatabaseURL)
	assert.Equal(t, 25, cfg.DatabaseMaxConns)
	assert.Equal(t, Pepper(testPepper), cfg.APIKeyPepper)
	assert.Equal(t, "http://collector:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "http://collector:4318/v1/traces", cfg.OTELExporterOTLPTracesEndpoint)
	assert.True(t, cfg.TracingEnabled())
}

func TestConfig_TracingEnabled(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		cfg  Config
		want bool
	}{
		"neither":         {cfg: Config{}, want: false},
		"blank":           {cfg: Config{OTELExporterOTLPEndpoint: " ", OTELExporterOTLPTracesEndpoint: "\t"}, want: false},
		"endpoint":        {cfg: Config{OTELExporterOTLPEndpoint: "http://collector:4318"}, want: true},
		"traces endpoint": {cfg: Config{OTELExporterOTLPTracesEndpoint: "http://collector:4318/v1/traces"}, want: true},
		"both": {
			cfg:  Config{OTELExporterOTLPEndpoint: "http://a:4318", OTELExporterOTLPTracesEndpoint: "http://b:4318"},
			want: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.TracingEnabled())
		})
	}
}

func TestLoad_MetricsOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")
	t.Setenv("API_KEY_PEPPER", testPepper)
	t.Setenv("METRICS_ADDR", "")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Empty(t, cfg.MetricsAddr)
	assert.False(t, cfg.MetricsEnabled())
}

func TestLoad_Invalid(t *testing.T) {
	tests := map[string]string{
		"LOG_LEVEL":          "loud",
		"TRUSTED_PROXIES":    "10.0.0.1,proxy.local",
		"DATABASE_URL":       "",
		"DATABASE_MAX_CONNS": "many",
		"API_KEY_PEPPER":     "",
	}

	for key, value := range tests {
		t.Run(key, func(t *testing.T) {
			// Valid required values, so each case fails only because of its own key.
			t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")
			t.Setenv("API_KEY_PEPPER", testPepper)
			t.Setenv(key, value)

			_, err := Load()
			assert.Error(t, err)
		})
	}
}

func TestLoad_ShortPepper(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/pkgsilo")
	t.Setenv("API_KEY_PEPPER", "too-short-to-be-a-pepper")

	_, err := Load()

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "too-short-to-be-a-pepper", "errors are logged, so they must not hold the secret")
}

func TestPrefixes_UnmarshalText(t *testing.T) {
	t.Parallel()
	var p Prefixes
	require.NoError(t, p.UnmarshalText([]byte(" 10.0.0.1, 172.16.5.4/12 ,::1,,::ffff:192.0.2.1")))

	assert.Equal(t, Prefixes{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("192.0.2.1/32"),
	}, p)
}

func TestPrefixes_UnmarshalTextInvalid(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"proxy.local", "10.0.0.0/33", "10.0.0.1:80"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			var p Prefixes
			assert.ErrorContains(t, p.UnmarshalText([]byte(input)), input)
		})
	}
}
