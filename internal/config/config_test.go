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
	if got.Pkgsite.BaseURL != "https://pkg.go.dev/v1beta" {
		t.Fatalf("BaseURL = %q, want default", got.Pkgsite.BaseURL)
	}
	if got.Pkgsite.HTTPTimeout != 10*time.Second {
		t.Fatalf("HTTPTimeout = %s, want 10s", got.Pkgsite.HTTPTimeout)
	}
	if got.Pkgsite.RedisURL != "" {
		t.Fatalf("RedisURL = %q, want empty", got.Pkgsite.RedisURL)
	}
	if got.Pkgsite.CacheDisabled {
		t.Fatal("CacheDisabled = true, want false")
	}
}

func TestReadOverrides(t *testing.T) {
	t.Parallel()

	got, err := read(env.Options{Environment: map[string]string{
		"PORT":                   "9090",
		"PKGSITE_BASE_URL":       "http://example.test",
		"PKGSITE_HTTP_TIMEOUT":   "250ms",
		"PKGSITE_REDIS_URL":      "redis://localhost:6379/0",
		"PKGSITE_CACHE_DISABLED": "true",
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
	if got.Pkgsite.HTTPTimeout != 250*time.Millisecond {
		t.Fatalf("HTTPTimeout = %s, want 250ms", got.Pkgsite.HTTPTimeout)
	}
	if got.Pkgsite.RedisURL != "redis://localhost:6379/0" {
		t.Fatalf("RedisURL = %q, want override", got.Pkgsite.RedisURL)
	}
	if !got.Pkgsite.CacheDisabled {
		t.Fatal("CacheDisabled = false, want true")
	}
}
