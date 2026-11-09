// Package config loads the application configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds the application configuration, loaded from environment variables.
type Config struct {
	// Env selects the runtime environment. "dev" enables human-friendly, colored logs.
	Env      string `env:"ENV"       envDefault:"prod"`
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
	// MetricsAddr is the address of the listener that serves the metrics, defaultMetricsAddr if
	// METRICS_ADDR is unset. Empty turns metrics off.
	MetricsAddr string     `env:"METRICS_ADDR"`
	LogLevel    slog.Level `env:"LOG_LEVEL"    envDefault:"info"`
	// TrustedProxies are the reverse proxies (IPs or CIDRs, comma-separated) whose
	// X-Forwarded-For header is used to determine the client IP. Empty: the header is ignored.
	TrustedProxies Prefixes `env:"TRUSTED_PROXIES"`
	// DatabaseURL is the PostgreSQL connection string, e.g. postgres://user:pass@host:5432/db.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	// DatabaseMaxConns caps the open connections in the pool.
	DatabaseMaxConns int `env:"DATABASE_MAX_CONNS" envDefault:"10"`
	// APIKeyPepper is the secret key of the stored API key hashes. Changing it invalidates every API key.
	APIKeyPepper Pepper `env:"API_KEY_PEPPER,notEmpty"`
	// OTELExporterOTLPEndpoint and OTELExporterOTLPTracesEndpoint are the standard OpenTelemetry variables
	// of the OTLP exporter's endpoint. Either turns tracing on. The exporter reads them, and every other
	// OTEL_* variable, itself.
	OTELExporterOTLPEndpoint       string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTELExporterOTLPTracesEndpoint string `env:"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"`
	// OTELSDKDisabled and OTELTracesExporter are the standard OpenTelemetry variables that turn tracing off
	// despite an endpoint: OTEL_SDK_DISABLED=true and OTEL_TRACES_EXPORTER=none. Metrics stay on, since
	// METRICS_ADDR alone controls them.
	OTELSDKDisabled    string         `env:"OTEL_SDK_DISABLED"` //nolint:tagliatelle // Its initialisms run together.
	OTELTracesExporter TracesExporter `env:"OTEL_TRACES_EXPORTER"`
}

// defaultMetricsAddr is the address of the metrics listener if METRICS_ADDR is unset.
const defaultMetricsAddr = ":9090"

// Load parses the configuration from the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, err
	}

	// Not an envDefault, which env also applies to an empty value, the one that turns metrics off.
	if _, set := os.LookupEnv("METRICS_ADDR"); !set {
		cfg.MetricsAddr = defaultMetricsAddr
	}

	return cfg, nil
}

// MetricsEnabled reports whether the application serves metrics.
func (c Config) MetricsEnabled() bool {
	return c.MetricsAddr != ""
}

// TracingEnabled reports whether the application exports spans: whether one of the OTLP endpoints is set,
// unless OTEL_SDK_DISABLED is true, case-insensitively as the OpenTelemetry spec says, or
// OTEL_TRACES_EXPORTER is none. Like the exporter, it ignores blank values.
func (c Config) TracingEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(c.OTELSDKDisabled), "true") || c.OTELTracesExporter == TracesExporterNone {
		return false
	}

	return strings.TrimSpace(c.OTELExporterOTLPEndpoint) != "" ||
		strings.TrimSpace(c.OTELExporterOTLPTracesEndpoint) != ""
}

// TracesExporter is the value of OTEL_TRACES_EXPORTER: TracesExporterOTLP, the default, or TracesExporterNone.
// pkgsilo has no other exporter.
type TracesExporter string

const (
	// TracesExporterOTLP exports spans over OTLP/HTTP.
	TracesExporterOTLP TracesExporter = "otlp"
	// TracesExporterNone exports no spans, which turns tracing off.
	TracesExporterNone TracesExporter = "none"
)

// UnmarshalText parses otlp or none, case-insensitively. A blank value is the default, as if unset.
func (e *TracesExporter) UnmarshalText(text []byte) error {
	value := TracesExporter(strings.ToLower(strings.TrimSpace(string(text))))
	switch value {
	case "", TracesExporterOTLP, TracesExporterNone:
		*e = value

		return nil
	default:
		return fmt.Errorf("unsupported exporter %q: only %s and %s are supported", text, TracesExporterOTLP,
			TracesExporterNone)
	}
}

// IsDev reports whether the application runs in the development environment.
func (c Config) IsDev() bool {
	return c.Env == "dev"
}

// minPepperLength is the least number of bytes of a Pepper. Hex-encoded random bytes, as from
// `openssl rand -hex 32`, then hold at least 128 bits.
const minPepperLength = 32

// Pepper is a secret key for hashing. Its value is used as raw bytes, whatever its encoding.
type Pepper []byte

// UnmarshalText parses a pepper of at least minPepperLength bytes. Its error doesn't hold the value.
func (p *Pepper) UnmarshalText(text []byte) error {
	if len(text) < minPepperLength {
		return fmt.Errorf("must be at least %d characters long", minPepperLength)
	}

	*p = slices.Clone(text)

	return nil
}

// Prefixes is a list of IP prefixes, parsed from a comma-separated list of IPs and CIDRs.
type Prefixes []netip.Prefix

// UnmarshalText parses a comma-separated list of IPs and CIDRs, e.g. "10.0.0.1, 172.16.0.0/12".
// A bare IP becomes a single-address prefix, and IPv4-mapped IPv6 addresses are unmapped.
func (p *Prefixes) UnmarshalText(text []byte) error {
	var prefixes Prefixes
	for entry := range strings.SplitSeq(string(text), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		prefix, err := parsePrefix(entry)
		if err != nil {
			return fmt.Errorf("invalid prefix %q: %w", entry, err)
		}
		prefixes = append(prefixes, prefix)
	}

	*p = prefixes

	return nil
}

func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		return prefix.Masked(), err
	}

	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parse address: %w", err)
	}
	addr = addr.Unmap().WithZone("")

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
