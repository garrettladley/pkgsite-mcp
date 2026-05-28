package config

import (
	"testing"
	"time"
)

func TestReadDefaults(t *testing.T) {
	t.Parallel()

	got, err := read(mapGetenv(map[string]string{}))
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
	if got.KV.RedisPool.Size != 4 {
		t.Fatalf("RedisPool.Size = %d, want 4", got.KV.RedisPool.Size)
	}
	if got.KV.RedisPool.MinIdleConns != 2 {
		t.Fatalf("RedisPool.MinIdleConns = %d, want 2", got.KV.RedisPool.MinIdleConns)
	}
	if got.KV.RedisPool.MaxIdleConns != 4 {
		t.Fatalf("RedisPool.MaxIdleConns = %d, want 4", got.KV.RedisPool.MaxIdleConns)
	}
	if got.KV.RedisPool.MaxActiveConns != 8 {
		t.Fatalf("RedisPool.MaxActiveConns = %d, want 8", got.KV.RedisPool.MaxActiveConns)
	}
	if got.KV.RedisPool.Timeout != 250*time.Millisecond {
		t.Fatalf("RedisPool.Timeout = %s, want 250ms", got.KV.RedisPool.Timeout)
	}
	if got.KV.RedisTimeouts.Dial != time.Second {
		t.Fatalf("RedisTimeouts.Dial = %s, want 1s", got.KV.RedisTimeouts.Dial)
	}
	if got.KV.RedisTimeouts.Read != 750*time.Millisecond {
		t.Fatalf("RedisTimeouts.Read = %s, want 750ms", got.KV.RedisTimeouts.Read)
	}
	if got.KV.RedisTimeouts.Write != 750*time.Millisecond {
		t.Fatalf("RedisTimeouts.Write = %s, want 750ms", got.KV.RedisTimeouts.Write)
	}
	if got.KV.RedisConnMaxIdle != 10*time.Minute {
		t.Fatalf("RedisConnMaxIdle = %s, want 10m", got.KV.RedisConnMaxIdle)
	}
	if !got.KV.RedisDisableIdentity {
		t.Fatal("RedisDisableIdentity = false, want true")
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

	got, err := read(mapGetenv(map[string]string{
		"PORT":                        "9090",
		"O11Y_SERVICE_NAME":           "pkgsite-test",
		"O11Y_ENVIRONMENT":            "test",
		"O11Y_FLUSH_TIMEOUT":          "5s",
		"O11Y_TRACES_SAMPLE_RATE":     "0.25",
		"O11Y_ENABLE_LOGS":            "false",
		"O11Y_ENABLE_METRICS":         "false",
		"PKGSITE_BASE_URL":            "http://example.test",
		"PKGSITE_HTTP_TIMEOUT":        "250ms",
		"KV_REDIS_URL":                "redis://localhost:6379/0",
		"KV_REDIS_POOL_SIZE":          "12",
		"KV_REDIS_MIN_IDLE_CONNS":     "3",
		"KV_REDIS_MAX_IDLE_CONNS":     "6",
		"KV_REDIS_MAX_ACTIVE_CONNS":   "18",
		"KV_REDIS_POOL_TIMEOUT":       "400ms",
		"KV_REDIS_DIAL_TIMEOUT":       "2s",
		"KV_REDIS_READ_TIMEOUT":       "900ms",
		"KV_REDIS_WRITE_TIMEOUT":      "950ms",
		"KV_REDIS_CONN_MAX_IDLE_TIME": "12m",
		"KV_REDIS_DISABLE_IDENTITY":   "false",
		"PKGSITE_CACHE_DISABLED":      "true",
		"RATE_LIMIT_ENABLED":          "false",
		"RATE_LIMIT_REQUESTS":         "10",
		"RATE_LIMIT_WINDOW":           "30s",
		"SENTRY_DSN":                  "https://public@example.invalid/1",
	}))
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
	if got.KV.RedisPool.Size != 12 {
		t.Fatalf("RedisPool.Size = %d, want 12", got.KV.RedisPool.Size)
	}
	if got.KV.RedisPool.MinIdleConns != 3 {
		t.Fatalf("RedisPool.MinIdleConns = %d, want 3", got.KV.RedisPool.MinIdleConns)
	}
	if got.KV.RedisPool.MaxIdleConns != 6 {
		t.Fatalf("RedisPool.MaxIdleConns = %d, want 6", got.KV.RedisPool.MaxIdleConns)
	}
	if got.KV.RedisPool.MaxActiveConns != 18 {
		t.Fatalf("RedisPool.MaxActiveConns = %d, want 18", got.KV.RedisPool.MaxActiveConns)
	}
	if got.KV.RedisPool.Timeout != 400*time.Millisecond {
		t.Fatalf("RedisPool.Timeout = %s, want 400ms", got.KV.RedisPool.Timeout)
	}
	if got.KV.RedisTimeouts.Dial != 2*time.Second {
		t.Fatalf("RedisTimeouts.Dial = %s, want 2s", got.KV.RedisTimeouts.Dial)
	}
	if got.KV.RedisTimeouts.Read != 900*time.Millisecond {
		t.Fatalf("RedisTimeouts.Read = %s, want 900ms", got.KV.RedisTimeouts.Read)
	}
	if got.KV.RedisTimeouts.Write != 950*time.Millisecond {
		t.Fatalf("RedisTimeouts.Write = %s, want 950ms", got.KV.RedisTimeouts.Write)
	}
	if got.KV.RedisConnMaxIdle != 12*time.Minute {
		t.Fatalf("RedisConnMaxIdle = %s, want 12m", got.KV.RedisConnMaxIdle)
	}
	if got.KV.RedisDisableIdentity {
		t.Fatal("RedisDisableIdentity = true, want false")
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

func TestReadParseError(t *testing.T) {
	t.Parallel()

	_, err := read(mapGetenv(map[string]string{"RATE_LIMIT_REQUESTS": "lots"}))
	if err == nil {
		t.Fatal("Read returned nil error, want parse error")
	}
	const want = `config: parsing RATE_LIMIT_REQUESTS="lots": strconv.Atoi: parsing "lots": invalid syntax`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestReadParseErrorReportsAllFailures(t *testing.T) {
	t.Parallel()

	_, err := read(mapGetenv(map[string]string{
		"O11Y_FLUSH_TIMEOUT":   "soon",
		"PKGSITE_HTTP_TIMEOUT": "later",
		"KV_REDIS_POOL_SIZE":   "several",
		"RATE_LIMIT_REQUESTS":  "lots",
	}))
	if err == nil {
		t.Fatal("Read returned nil error, want parse error")
	}
	const want = `config: parsing KV_REDIS_POOL_SIZE="several": strconv.Atoi: parsing "several": invalid syntax
config: parsing O11Y_FLUSH_TIMEOUT="soon": time: invalid duration "soon"
config: parsing PKGSITE_HTTP_TIMEOUT="later": time: invalid duration "later"
config: parsing RATE_LIMIT_REQUESTS="lots": strconv.Atoi: parsing "lots": invalid syntax`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func mapGetenv(env map[string]string) func(string) string {
	return func(key string) string {
		return env[key]
	}
}
