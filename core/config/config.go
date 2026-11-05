// Package config loads the application configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds the application configuration, loaded from environment variables.
type Config struct {
	// Env selects the runtime environment. "dev" enables human-friendly, colored logs.
	Env      string     `env:"ENV"       envDefault:"prod"`
	HTTPAddr string     `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`
	// TrustedProxies are the reverse proxies (IPs or CIDRs, comma-separated) whose
	// X-Forwarded-For header is used to determine the client IP. Empty: the header is ignored.
	TrustedProxies Prefixes `env:"TRUSTED_PROXIES"`
	// DatabaseURL is the PostgreSQL connection string, e.g. postgres://user:pass@host:5432/db.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	// DatabaseMaxConns caps the open connections in the pool.
	DatabaseMaxConns int `env:"DATABASE_MAX_CONNS" envDefault:"10"`
	// APIKeyPepper is the secret key of the stored API key hashes. Changing it invalidates every API key.
	APIKeyPepper Pepper `env:"API_KEY_PEPPER,notEmpty"`
}

// Load parses the configuration from the process environment.
func Load() (Config, error) {
	return env.ParseAs[Config]()
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
