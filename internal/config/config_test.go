package config

import (
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
)

func TestReadDefaults(t *testing.T) {
	t.Parallel()

	got, err := read(env.Options{Environment: map[string]string{}})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", got.Port)
	}
	if got.Observability.ServiceName != "pkgsite-mcp" {
		t.Fatalf("ServiceName = %q, want pkgsite-mcp", got.Observability.ServiceName)
	}
	if got.Observability.FlushTimeout != 2*time.Second {
		t.Fatalf("FlushTimeout = %s, want 2s", got.Observability.FlushTimeout)
	}
	if got.Pkgsite.BaseURL != "https://pkg.go.dev/v1beta" {
		t.Fatalf("BaseURL = %q, want default", got.Pkgsite.BaseURL)
	}
	if got.Pkgsite.HTTPTimeout != 10*time.Second {
		t.Fatalf("HTTPTimeout = %s, want 10s", got.Pkgsite.HTTPTimeout)
	}
	if got.KV.RedisURL != "" {
		t.Fatalf("RedisURL = %q, want empty", got.KV.RedisURL)
	}
	if got.Pkgsite.CacheDisabled {
		t.Fatal("CacheDisabled = true, want false")
	}
	if !got.RateLimit.Enabled {
		t.Fatal("RateLimit.Enabled = false, want true")
	}
	if got.RateLimit.Requests != 120 {
		t.Fatalf("RateLimit.Requests = %d, want 120", got.RateLimit.Requests)
	}
	if got.RateLimit.Window != time.Minute {
		t.Fatalf("RateLimit.Window = %s, want 1m", got.RateLimit.Window)
	}
	if got.Observability.TracesSampleRate != 1.0 {
		t.Fatalf("TracesSampleRate = %f, want 1.0", got.Observability.TracesSampleRate)
	}
	if !got.Observability.EnableLogs {
		t.Fatal("EnableLogs = false, want true")
	}
	if !got.Observability.EnableMetrics {
		t.Fatal("EnableMetrics = false, want true")
	}
}

func TestReadOverrides(t *testing.T) {
	t.Parallel()

	got, err := read(env.Options{Environment: map[string]string{
		"PORT":                    "9090",
		"O11Y_SERVICE_NAME":       "pkgsite-test",
		"O11Y_ENVIRONMENT":        "test",
		"O11Y_FLUSH_TIMEOUT":      "5s",
		"O11Y_TRACES_SAMPLE_RATE": "0.25",
		"O11Y_ENABLE_LOGS":        "false",
		"O11Y_ENABLE_METRICS":     "false",
		"PKGSITE_BASE_URL":        "http://example.test",
		"PKGSITE_HTTP_TIMEOUT":    "250ms",
		"KV_REDIS_URL":            "redis://localhost:6379/0",
		"PKGSITE_CACHE_DISABLED":  "true",
		"RATE_LIMIT_ENABLED":      "false",
		"RATE_LIMIT_REQUESTS":     "10",
		"RATE_LIMIT_WINDOW":       "30s",
		"SENTRY_DSN":              "https://public@example.invalid/1",
	}})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got.HTTPAddr("") != ":9090" {
		t.Fatalf("HTTPAddr(\"\") = %q, want :9090", got.HTTPAddr(""))
	}
	if got.HTTPAddr("9091") != ":9091" {
		t.Fatalf("HTTPAddr(\"9091\") = %q, want :9091", got.HTTPAddr("9091"))
	}
	if got.HTTPAddr("127.0.0.1:9092") != "127.0.0.1:9092" {
		t.Fatalf("HTTPAddr(\"127.0.0.1:9092\") = %q, want 127.0.0.1:9092", got.HTTPAddr("127.0.0.1:9092"))
	}
	if got.Pkgsite.BaseURL != "http://example.test" {
		t.Fatalf("BaseURL = %q, want override", got.Pkgsite.BaseURL)
	}
	if got.Observability.ServiceName != "pkgsite-test" {
		t.Fatalf("ServiceName = %q, want pkgsite-test", got.Observability.ServiceName)
	}
	if got.Observability.Environment != "test" {
		t.Fatalf("Environment = %q, want test", got.Observability.Environment)
	}
	if got.Observability.FlushTimeout != 5*time.Second {
		t.Fatalf("FlushTimeout = %s, want 5s", got.Observability.FlushTimeout)
	}
	if got.Pkgsite.HTTPTimeout != 250*time.Millisecond {
		t.Fatalf("HTTPTimeout = %s, want 250ms", got.Pkgsite.HTTPTimeout)
	}
	if got.KV.RedisURL != "redis://localhost:6379/0" {
		t.Fatalf("RedisURL = %q, want override", got.KV.RedisURL)
	}
	if !got.Pkgsite.CacheDisabled {
		t.Fatal("CacheDisabled = false, want true")
	}
	if got.Sentry.DSN != "https://public@example.invalid/1" {
		t.Fatalf("Sentry.DSN = %q, want override", got.Sentry.DSN)
	}
	if got.RateLimit.Enabled {
		t.Fatal("RateLimit.Enabled = true, want false")
	}
	if got.RateLimit.Requests != 10 {
		t.Fatalf("RateLimit.Requests = %d, want 10", got.RateLimit.Requests)
	}
	if got.RateLimit.Window != 30*time.Second {
		t.Fatalf("RateLimit.Window = %s, want 30s", got.RateLimit.Window)
	}
	if got.Observability.TracesSampleRate != 0.25 {
		t.Fatalf("TracesSampleRate = %f, want 0.25", got.Observability.TracesSampleRate)
	}
	if got.Observability.EnableLogs {
		t.Fatal("EnableLogs = true, want false")
	}
	if got.Observability.EnableMetrics {
		t.Fatal("EnableMetrics = true, want false")
	}
}
