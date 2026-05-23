package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
	"go.opentelemetry.io/otel/trace"
)

func RateLimit(store kv.Store, cfg config.RateLimit, logger *slog.Logger) Middleware {
	if !cfg.Enabled || cfg.Requests <= 0 || cfg.Window <= 0 || store == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/mcp" {
				trace.SpanFromContext(r.Context()).SetAttributes(observability.RateLimitAttrs{Outcome: observability.RateLimitOutcomeSkipped, Limit: cfg.Requests, Window: cfg.Window}.Attributes()...)
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			now := time.Now()
			reset := rateLimitReset(cfg.Window, now)
			key := rateLimitKey(ip, cfg.Window, now)
			count, err := store.Increment(r.Context(), key, cfg.Window+time.Second)
			if err != nil {
				trace.SpanFromContext(r.Context()).SetAttributes(observability.RateLimitAttrs{Outcome: observability.RateLimitOutcomeStoreError, Limit: cfg.Requests, Window: cfg.Window}.Attributes()...)
				logger.ErrorContext(r.Context(), "rate limit check failed", slog.Any("error", err), slog.String("client_ip", ip))
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
			remaining := max(int64(cfg.Requests)-count, 0)
			w.Header().Set(xhttp.HeaderXRateLimitLimit, strconv.Itoa(cfg.Requests))
			w.Header().Set(xhttp.HeaderXRateLimitRemaining, strconv.FormatInt(remaining, 10))
			w.Header().Set(xhttp.HeaderXRateLimitReset, strconv.FormatInt(reset.Unix(), 10))
			if count > int64(cfg.Requests) {
				trace.SpanFromContext(r.Context()).SetAttributes(observability.RateLimitAttrs{Outcome: observability.RateLimitOutcomeLimited, RemainingBucket: observability.RemainingBucket(remaining, cfg.Requests), Limit: cfg.Requests, Window: cfg.Window}.Attributes()...)
				w.Header().Set(xhttp.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds(reset, now)))
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			trace.SpanFromContext(r.Context()).SetAttributes(observability.RateLimitAttrs{Outcome: observability.RateLimitOutcomeAllowed, RemainingBucket: observability.RemainingBucket(remaining, cfg.Requests), Limit: cfg.Requests, Window: cfg.Window}.Attributes()...)
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitKey(ip string, window time.Duration, now time.Time) string {
	sum := sha256.Sum256([]byte(ip))
	bucket := now.UnixNano() / window.Nanoseconds()
	return fmt.Sprintf("pkgsite:mcp:ratelimit:ip:%s:%d", hex.EncodeToString(sum[:]), bucket)
}

func rateLimitReset(window time.Duration, now time.Time) time.Time {
	bucket := now.UnixNano() / window.Nanoseconds()
	return time.Unix(0, (bucket+1)*window.Nanoseconds())
}

func retryAfterSeconds(reset, now time.Time) int {
	seconds := int(reset.Sub(now).Round(time.Second).Seconds())
	return max(seconds, 1)
}

func clientIP(r *http.Request) string {
	for _, value := range []string{
		r.Header.Get(xhttp.HeaderFlyClientIP),
		firstForwardedFor(r.Header.Get(xhttp.HeaderXForwardedFor)),
		r.Header.Get(xhttp.HeaderXRealIP),
		r.RemoteAddr,
	} {
		if ip := normalizeIP(value); ip != "" {
			return ip
		}
	}
	return "unknown"
}

func firstForwardedFor(value string) string {
	ip, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(ip)
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return ""
	}
	return ip.Unmap().String()
}
