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

	"github.com/redis/go-redis/v9"
)

const cacheHitHeader = "X-Pkgsite-Mcp-Cache-Hit"

type cachedDoer struct {
	client *http.Client
	redis  *redis.Client
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
			base: http.DefaultTransport,
		},
		CheckRedirect: base.CheckRedirect,
	}
}

func newCachedDoer(httpClient *http.Client, redisURL string, disabled bool) (*cachedDoer, error) {
	doer := &cachedDoer{client: httpClient, off: disabled || redisURL == ""}
	if doer.off {
		return doer, nil
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	doer.redis = redis.NewClient(opts)
	return doer, nil
}

func (d *cachedDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet || d.off || d.redis == nil {
		return d.client.Do(req)
	}

	key := cacheKey(req)
	if cached, err := d.redis.Get(req.Context(), key).Bytes(); err == nil {
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
			}, nil
		}
		// Backward-compatible fallback for early body-only cache entries.
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				cacheHitHeader: []string{"true"},
			},
			Body:    io.NopCloser(bytes.NewReader(cached)),
			Request: req,
		}
		return resp, nil
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
			_ = d.redis.Set(req.Context(), key, encoded, ttl).Err()
		}
	}
	return resp, nil
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
