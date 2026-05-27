package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
)

func TestRateLimitAllowsWithinLimit(t *testing.T) {
	t.Parallel()

	store := incrementFunc(func(context.Context, string, time.Duration) (int64, error) {
		return 2, nil
	})
	handler := RateLimit(store, config.RateLimit{Enabled: true, Requests: 2, Window: time.Minute}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, requestWithIP(t, "203.0.113.10:1234"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get(xhttp.HeaderXRateLimitRemaining); got != "0" {
		t.Fatalf("remaining = %q, want 0", got)
	}
}

func TestRateLimitRejectsOverLimit(t *testing.T) {
	t.Parallel()

	store := incrementFunc(func(context.Context, string, time.Duration) (int64, error) {
		return 3, nil
	})
	handler := RateLimit(store, config.RateLimit{Enabled: true, Requests: 2, Window: time.Minute}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, requestWithIP(t, "203.0.113.10:1234"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get(xhttp.HeaderRetryAfter))
	if err != nil {
		t.Fatalf("Retry-After is not an integer: %q", rec.Header().Get(xhttp.HeaderRetryAfter))
	}
	if retryAfter < 1 || retryAfter > 60 {
		t.Fatalf("Retry-After = %d, want 1..60", retryAfter)
	}
}

func TestRateLimitFailsClosedOnStoreError(t *testing.T) {
	t.Parallel()

	store := incrementFunc(func(context.Context, string, time.Duration) (int64, error) {
		return 0, errors.New("redis down")
	})
	handler := RateLimit(store, config.RateLimit{Enabled: true, Requests: 2, Window: time.Minute}, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, requestWithIP(t, "203.0.113.10:1234"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRateLimitDoesNotLogEndedRequestsAsStoreErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "canceled", err: context.Canceled},
		{name: "deadline_exceeded", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var errorLogs atomic.Int64
			logger := slog.New(countingErrorHandler{count: &errorLogs})
			store := incrementFunc(func(context.Context, string, time.Duration) (int64, error) {
				return 0, tt.err
			})
			handler := RateLimit(store, config.RateLimit{Enabled: true, Requests: 2, Window: time.Minute}, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, requestWithIP(t, "203.0.113.10:1234"))

			if got := errorLogs.Load(); got != 0 {
				t.Fatalf("error logs = %d, want 0", got)
			}
		})
	}
}

func TestRateLimitContextOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want observability.RateLimitOutcome
		ok   bool
	}{
		{name: "canceled", err: context.Canceled, want: observability.RateLimitOutcomeCanceled, ok: true},
		{name: "deadline_exceeded", err: context.DeadlineExceeded, want: observability.RateLimitOutcomeDeadline, ok: true},
		{name: "other", err: errors.New("redis down"), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := rateLimitContextOutcome(tt.err)
			if ok != tt.ok {
				t.Fatalf("ok = %t, want %t", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("outcome = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIPPrefersFlyHeaderAndNormalizes(t *testing.T) {
	t.Parallel()

	req := requestWithIP(t, "198.51.100.10:1234")
	req.Header.Set(xhttp.HeaderFlyClientIP, "::ffff:203.0.113.9")
	req.Header.Set(xhttp.HeaderXForwardedFor, "192.0.2.1, 198.51.100.10")

	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("clientIP = %q, want 203.0.113.9", got)
	}
}

func requestWithIP(t *testing.T, remoteAddr string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.test/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = remoteAddr
	return req
}

type incrementFunc func(context.Context, string, time.Duration) (int64, error)

func (f incrementFunc) Get(context.Context, string) ([]byte, error) {
	return nil, kv.ErrNotFound
}

func (f incrementFunc) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (f incrementFunc) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return f(ctx, key, ttl)
}

type countingErrorHandler struct {
	count *atomic.Int64
}

var _ slog.Handler = countingErrorHandler{}

func (h countingErrorHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelError
}

func (h countingErrorHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		h.count.Add(1)
	}
	return nil
}

func (h countingErrorHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h countingErrorHandler) WithGroup(string) slog.Handler {
	return h
}
