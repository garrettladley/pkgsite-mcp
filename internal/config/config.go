package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	KV            KV
	Observability Observability
	Pkgsite       Pkgsite
	RateLimit     RateLimit
	Sentry        Sentry
}

type Pkgsite struct {
	BaseURL       string
	HTTPTimeout   time.Duration
	CacheDisabled bool
}

type KV struct {
	RedisURL string
}

type RateLimit struct {
	Enabled  bool
	Requests int
	Window   time.Duration
}

type Sentry struct {
	DSN string
}

type Observability struct {
	ServiceName      string
	Environment      string
	FlushTimeout     time.Duration
	TracesSampleRate float64
	EnableLogs       bool
	EnableMetrics    bool
}

// Read loads configuration from the process environment.
func Read() (Config, error) {
	return read(os.Getenv)
}

// read loads configuration using the given lookup. Injecting getenv (rather
// than calling os.Getenv directly) keeps parsing pure and lets tests run in
// parallel without mutating process-wide state.
func read(getenv func(string) string) (Config, error) {
	p := parser{getenv: getenv}

	cfg := Config{
		Port: p.str("PORT", "8080"),
		KV: KV{
			RedisURL: p.str("KV_REDIS_URL", ""),
		},
		Observability: Observability{
			ServiceName:      p.str("O11Y_SERVICE_NAME", "pkgsite-mcp"),
			Environment:      p.str("O11Y_ENVIRONMENT", ""),
			FlushTimeout:     p.duration("O11Y_FLUSH_TIMEOUT", 2*time.Second),
			TracesSampleRate: p.float("O11Y_TRACES_SAMPLE_RATE", 1.0),
			EnableLogs:       p.boolean("O11Y_ENABLE_LOGS", true),
			EnableMetrics:    p.boolean("O11Y_ENABLE_METRICS", true),
		},
		Pkgsite: Pkgsite{
			BaseURL:       p.str("PKGSITE_BASE_URL", "https://pkg.go.dev/v1beta"),
			HTTPTimeout:   p.duration("PKGSITE_HTTP_TIMEOUT", 10*time.Second),
			CacheDisabled: p.boolean("PKGSITE_CACHE_DISABLED", false),
		},
		RateLimit: RateLimit{
			Enabled:  p.boolean("RATE_LIMIT_ENABLED", true),
			Requests: p.intVal("RATE_LIMIT_REQUESTS", 120),
			Window:   p.duration("RATE_LIMIT_WINDOW", time.Minute),
		},
		Sentry: Sentry{
			DSN: p.str("SENTRY_DSN", ""),
		},
	}
	if p.err != nil {
		return Config{}, p.err
	}
	return cfg, nil
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

// parser reads typed values via getenv, applying a default when a key is
// unset/empty and recording the first parse failure.
type parser struct {
	getenv func(string) string
	err    error
}

func (p *parser) str(key, def string) string {
	if v := p.getenv(key); v != "" {
		return v
	}
	return def
}

func (p *parser) boolean(key string, def bool) bool {
	v := p.getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.record(key, v, err)
		return def
	}
	return b
}

func (p *parser) intVal(key string, def int) int {
	v := p.getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.record(key, v, err)
		return def
	}
	return n
}

func (p *parser) float(key string, def float64) float64 {
	v := p.getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.record(key, v, err)
		return def
	}
	return f
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	v := p.getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		p.record(key, v, err)
		return def
	}
	return d
}

func (p *parser) record(key, val string, err error) {
	if p.err == nil {
		p.err = fmt.Errorf("config: parsing %s=%q: %w", key, val, err)
	}
}
