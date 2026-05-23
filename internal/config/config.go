package config

import (
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port          string        `env:"PORT" envDefault:"8080"`
	KV            KV            `envPrefix:"KV_"`
	Observability Observability `envPrefix:"O11Y_"`
	Pkgsite       Pkgsite       `envPrefix:"PKGSITE_"`
	RateLimit     RateLimit     `envPrefix:"RATE_LIMIT_"`
	Sentry        Sentry
}

type Pkgsite struct {
	BaseURL       string        `env:"BASE_URL" envDefault:"https://pkg.go.dev/v1beta"`
	HTTPTimeout   time.Duration `env:"HTTP_TIMEOUT" envDefault:"10s"`
	CacheDisabled bool          `env:"CACHE_DISABLED" envDefault:"false"`
}

type KV struct {
	RedisURL string `env:"REDIS_URL"`
}

type RateLimit struct {
	Enabled  bool          `env:"ENABLED" envDefault:"true"`
	Requests int           `env:"REQUESTS" envDefault:"120"`
	Window   time.Duration `env:"WINDOW" envDefault:"1m"`
}

type Sentry struct {
	DSN string `env:"SENTRY_DSN"`
}

type Observability struct {
	ServiceName      string        `env:"SERVICE_NAME" envDefault:"pkgsite-mcp"`
	Environment      string        `env:"ENVIRONMENT"`
	FlushTimeout     time.Duration `env:"FLUSH_TIMEOUT" envDefault:"2s"`
	TracesSampleRate float64       `env:"TRACES_SAMPLE_RATE" envDefault:"1.0"`
	EnableLogs       bool          `env:"ENABLE_LOGS" envDefault:"true"`
	EnableMetrics    bool          `env:"ENABLE_METRICS" envDefault:"true"`
}

func Read() (Config, error) {
	return read(env.Options{})
}

func read(opts env.Options) (Config, error) {
	return env.ParseAsWithOptions[Config](opts)
}

func (c Config) HTTPAddr(override string) string {
	addr := strings.TrimSpace(override)
	if addr == "" {
		addr = ":" + strings.TrimSpace(c.Port)
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	return addr
}
