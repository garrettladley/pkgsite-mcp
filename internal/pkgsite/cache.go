package pkgsite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const cacheHitHeader = "X-Pkgsite-Mcp-Cache-Hit"

type cachedDoer struct {
	client *http.Client
	store  kv.Store
	off    bool
}

type cachedResponse struct {
	StatusCode int         `json:"statusCode"`
	Status     string      `json:"status"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
}

func newHTTPClient(timeout time.Duration) *http.Client {
	base := &http.Client{Timeout: timeout}
	return &http.Client{
		Timeout: timeout,
		Transport: retryTransport{
			base: otelhttp.NewTransport(http.DefaultTransport),
		},
		CheckRedirect: base.CheckRedirect,
	}
}

func newCachedDoer(httpClient *http.Client, store kv.Store, disabled bool) *cachedDoer {
	return &cachedDoer{client: httpClient, store: store, off: disabled || store == nil}
}

func (d *cachedDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet || d.off || d.store == nil {
		observability.RecordCacheLookup(req.Context(), cacheBypassOutcome(req, d), 0)
		return d.client.Do(req)
	}

	ctx, span := observability.Tracer("pkgsite-cache").Start(req.Context(), "pkgsite.cache lookup",
		trace.WithAttributes(
			attribute.String("http.request.method", req.Method),
			attribute.String("url.path", req.URL.EscapedPath()),
		),
	)
	defer span.End()
	req = req.WithContext(ctx)
	start := time.Now()
	key := cacheKey(req)
	span.SetAttributes(attribute.String("cache.key_hash", strings.TrimPrefix(key, "pkgsite:v1beta:http:")))
	cached, err := d.store.Get(req.Context(), key)
	switch {
	case err == nil:
		observability.RecordCacheLookup(req.Context(), observability.CacheOutcomeHit, time.Since(start))
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return cachedHTTPResponse(req, cached), nil
	case !errors.Is(err, kv.ErrNotFound):
		observability.RecordCacheLookup(req.Context(), observability.CacheOutcomeError, time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	default:
		observability.RecordCacheLookup(req.Context(), observability.CacheOutcomeMiss, time.Since(start))
		span.SetAttributes(attribute.Bool("cache.hit", false))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	if ttl := cacheTTL(req.URL, resp.StatusCode); ttl > 0 {
		record := cachedResponse{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Header:     resp.Header.Clone(),
			Body:       body,
		}
		if encoded, err := json.Marshal(record); err == nil {
			err = d.store.Set(req.Context(), key, encoded, ttl)
			observability.RecordCacheWrite(req.Context(), err == nil)
			if err != nil {
				span.RecordError(err)
			}
		} else {
			observability.RecordCacheWrite(req.Context(), false)
			span.RecordError(err)
		}
	}
	return resp, nil
}

func cachedHTTPResponse(req *http.Request, cached []byte) *http.Response {
	var record cachedResponse
	if err := json.Unmarshal(cached, &record); err == nil {
		if record.Header == nil {
			record.Header = http.Header{}
		}
		record.Header.Set(cacheHitHeader, "true")
		return &http.Response{
			StatusCode: record.StatusCode,
			Status:     record.Status,
			Header:     record.Header,
			Body:       io.NopCloser(bytes.NewReader(record.Body)),
			Request:    req,
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			cacheHitHeader: []string{"true"},
		},
		Body:    io.NopCloser(bytes.NewReader(cached)),
		Request: req,
	}
}

func cacheBypassOutcome(req *http.Request, d *cachedDoer) observability.CacheOutcome {
	if req.Method != http.MethodGet {
		return observability.CacheOutcomeBypass
	}
	if d.off || d.store == nil {
		return observability.CacheOutcomeDisabled
	}
	return observability.CacheOutcomeBypass
}

func cacheKey(req *http.Request) string {
	u := *req.URL
	q := u.Query()
	keys := make([]string, 0, len(q))
	for key := range q {
		sort.Strings(q[key])
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make(url.Values, len(q))
	for _, key := range keys {
		values[key] = q[key]
	}
	u.RawQuery = values.Encode()
	sum := sha256.Sum256([]byte(req.Method + " " + u.String()))
	return "pkgsite:v1beta:http:" + hex.EncodeToString(sum[:])
}

func cacheTTL(u *url.URL, status int) time.Duration {
	if status >= 500 {
		return time.Minute
	}
	if status >= 400 {
		return 5 * time.Minute
	}
	path := u.EscapedPath()
	version := strings.TrimSpace(u.Query().Get("version"))
	switch {
	case strings.Contains(path, "/search"):
		return 15 * time.Minute
	case strings.Contains(path, "/imported-by/"):
		return 6 * time.Hour
	case version == "" || version == "latest" || version == "main" || version == "master":
		return time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

type retryTransport struct {
	base http.RoundTripper
}

func (t retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt*150) * time.Millisecond)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}
		resp, err := base.RoundTrip(req)
		if err != nil {
			lastErr = err
			if errors.Is(err, context.Canceled) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 500 {
			return resp, nil
		}
		if attempt == 2 {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return nil, lastErr
}
