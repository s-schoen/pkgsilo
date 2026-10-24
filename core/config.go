package core

import (
	"log/slog"

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
	TrustedProxies TrustedProxies `env:"TRUSTED_PROXIES"`
	// DatabaseURL is the PostgreSQL connection string, e.g. postgres://user:pass@host:5432/db.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	// DatabaseMaxConns caps the open connections in the pool.
	DatabaseMaxConns int `env:"DATABASE_MAX_CONNS" envDefault:"10"`
}

// LoadConfig parses the configuration from the process environment.
func LoadConfig() (Config, error) {
	return env.ParseAs[Config]()
}

// IsDev reports whether the application runs in the development environment.
func (c Config) IsDev() bool {
	return c.Env == "dev"
}
