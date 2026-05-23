package config

import (
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port    string  `env:"PORT" envDefault:"8080"`
	Pkgsite Pkgsite `envPrefix:"PKGSITE_"`
	Sentry  Sentry
}

type Pkgsite struct {
	BaseURL       string        `env:"BASE_URL" envDefault:"https://pkg.go.dev/v1beta"`
	HTTPTimeout   time.Duration `env:"HTTP_TIMEOUT" envDefault:"10s"`
	RedisURL      string        `env:"REDIS_URL"`
	CacheDisabled bool          `env:"CACHE_DISABLED" envDefault:"false"`
}

type Sentry struct {
	DSN string `env:"SENTRY_DSN"`
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
