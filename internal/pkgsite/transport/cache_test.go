package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/kv"
)

func TestCacheKeyNormalizesQueryOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		first  string
		second string
	}{
		{
			name:   "query order",
			first:  "https://pkg.go.dev/v1beta/symbols/golang.org/x/oauth2?version=v0.35.0&limit=10",
			second: "https://pkg.go.dev/v1beta/symbols/golang.org/x/oauth2?limit=10&version=v0.35.0",
		},
		{
			name:   "repeated query values",
			first:  "https://pkg.go.dev/v1beta/search?filter=b&filter=a&q=uuid",
			second: "https://pkg.go.dev/v1beta/search?q=uuid&filter=a&filter=b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			first := requestFor(t, tt.first)
			second := requestFor(t, tt.second)
			if cacheKey(first) != cacheKey(second) {
				t.Fatalf("cache keys differ for semantically equivalent URLs")
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		status int
	}{
		{name: "search success", rawURL: "https://pkg.go.dev/v1beta/search?q=uuid", status: http.StatusOK},
		{name: "not found", rawURL: "https://pkg.go.dev/v1beta/module/example.com/nope", status: http.StatusNotFound},
		{name: "version pinned", rawURL: "https://pkg.go.dev/v1beta/module/golang.org/x/oauth2?version=v0.35.0", status: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := parseURLForTest(t, tt.rawURL)
			if got := cacheTTL(u, tt.status); got == 0 {
				t.Fatal("TTL is zero")
			}
		})
	}

	if got := cacheTTL(parseURLForTest(t, "https://pkg.go.dev/v1/search?q=uuid"), http.StatusTooManyRequests); got != 0 {
		t.Fatalf("rate-limited response TTL = %s, want 0", got)
	}
}

func TestRedisRateLimitedTransportHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})
	store := &incrementStore{count: upstreamRequestsPerSecond + 1}
	transport := &redisRateLimitedTransport{
		base:  base,
		store: store,
		limit: upstreamRequestsPerSecond,
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	request := requestFor(t, "https://pkg.go.dev/v1/search?q=second").WithContext(ctx)
	response, err := transport.RoundTrip(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RoundTrip error = %v, want context canceled", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("base RoundTrip calls = %d, want 0", got)
	}
}

func TestRedisRateLimitedTransportUsesSharedStore(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    req,
		}, nil
	})
	store := &incrementStore{count: 1}
	transport := &redisRateLimitedTransport{base: base, store: store, limit: upstreamRequestsPerSecond}

	response, err := transport.RoundTrip(requestFor(t, "https://pkg.go.dev/v1/search?q=shared"))
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	_ = response.Body.Close()
	if got := calls.Load(); got != 1 {
		t.Fatalf("base RoundTrip calls = %d, want 1", got)
	}
	if !strings.HasPrefix(store.key, upstreamRateLimitKeyPrefix) {
		t.Fatalf("Increment key = %q, want prefix %q", store.key, upstreamRateLimitKeyPrefix)
	}
	if store.ttl <= time.Second {
		t.Fatalf("Increment TTL = %s, want more than one second", store.ttl)
	}
}

func TestRedisRateLimitedTransportPropagatesStoreErrors(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("redis unavailable")
	store := &incrementStore{err: storeErr}
	transport := &redisRateLimitedTransport{
		store: store,
		limit: upstreamRequestsPerSecond,
	}

	response, err := transport.RoundTrip(requestFor(t, "https://pkg.go.dev/v1/search?q=error"))
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, storeErr) {
		t.Fatalf("RoundTrip error = %v, want %v", err, storeErr)
	}
}

func TestRedisRateLimitedTransportDoesNotUseLocalFallback(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	transport := &redisRateLimitedTransport{
		base: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Request:    req,
			}, nil
		}),
		limit: upstreamRequestsPerSecond,
	}

	response, err := transport.RoundTrip(requestFor(t, "https://pkg.go.dev/v1/search?q=no-redis"))
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	_ = response.Body.Close()
	if got := calls.Load(); got != 1 {
		t.Fatalf("base RoundTrip calls = %d, want 1", got)
	}
}

func TestCachedDoerCoalescesConcurrentMisses(t *testing.T) {
	t.Parallel()

	const callers = 50
	payload := []byte(`{"name":"golang.org/x/sync"}`)
	store := newCountingStore(callers)
	upstream := &gatedRoundTripper{
		statusCode: http.StatusOK,
		status:     "200 OK",
		body:       payload,
		release:    make(chan struct{}),
	}
	doer := NewCachedDoer(&http.Client{Transport: upstream}, store, false)

	type result struct {
		body []byte
		err  error
	}
	results := make([]result, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			resp, err := doer.Do(requestFor(t, "https://pkg.go.dev/v1beta/module/golang.org/x/sync?version=v0.20.0"))
			if err != nil {
				results[i].err = err
				return
			}
			defer func() {
				_ = resp.Body.Close()
			}()
			results[i].body, results[i].err = io.ReadAll(resp.Body)
		}(i)
	}

	close(start)
	store.waitForGets(t)
	close(upstream.release)
	wg.Wait()

	upstreamCalls := upstream.calls.Load()
	if upstreamCalls >= callers {
		t.Fatalf("upstream calls = %d, want fewer than %d", upstreamCalls, callers)
	}
	if got := store.sets.Load(); got != upstreamCalls {
		t.Fatalf("store sets = %d, want %d", got, upstreamCalls)
	}
	for i, result := range results {
		if result.err != nil {
			t.Fatalf("result %d error: %v", i, result.err)
		}
		if !bytes.Equal(result.body, payload) {
			t.Fatalf("result %d body = %q, want %q", i, result.body, payload)
		}
	}
}

func TestCachedDoerCoalescesConcurrentMissErrors(t *testing.T) {
	t.Parallel()

	const callers = 50
	store := newCountingStore(callers)
	upstreamErr := errors.New("upstream unavailable")
	upstream := &gatedRoundTripper{
		err:     upstreamErr,
		release: make(chan struct{}),
	}
	doer := NewCachedDoer(&http.Client{Transport: upstream}, store, false)

	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			resp, err := doer.Do(requestFor(t, "https://pkg.go.dev/v1beta/module/golang.org/x/sync?version=v0.20.0"))
			if resp != nil {
				_ = resp.Body.Close()
			}
			errs[i] = err
		}(i)
	}

	close(start)
	store.waitForGets(t)
	close(upstream.release)
	wg.Wait()

	if got := upstream.calls.Load(); got >= callers {
		t.Fatalf("upstream calls = %d, want fewer than %d", got, callers)
	}
	if got := store.sets.Load(); got != 0 {
		t.Fatalf("store sets = %d, want 0", got)
	}
	for i, err := range errs {
		if err == nil {
			t.Fatalf("result %d error = nil, want error", i)
		}
	}
}

func requestFor(t testing.TB, raw string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func parseURLForTest(t testing.TB, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

type countingStore struct {
	targetGets int64
	gets       atomic.Int64
	sets       atomic.Int64
	ready      chan struct{}
	readyOnce  sync.Once
}

type incrementStore struct {
	count int64
	err   error
	key   string
	ttl   time.Duration
}

func (s *incrementStore) Get(context.Context, string) ([]byte, error) {
	return nil, kv.ErrNotFound
}

func (s *incrementStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (s *incrementStore) Increment(_ context.Context, key string, ttl time.Duration) (int64, error) {
	s.key = key
	s.ttl = ttl
	return s.count, s.err
}

func newCountingStore(targetGets int64) *countingStore {
	return &countingStore{
		targetGets: targetGets,
		ready:      make(chan struct{}),
	}
}

func (s *countingStore) Get(context.Context, string) ([]byte, error) {
	if s.gets.Add(1) == s.targetGets {
		s.readyOnce.Do(func() { close(s.ready) })
	}
	return nil, kv.ErrNotFound
}

func (s *countingStore) Set(context.Context, string, []byte, time.Duration) error {
	s.sets.Add(1)
	return nil
}

func (s *countingStore) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, nil
}

func (s *countingStore) waitForGets(t testing.TB) {
	t.Helper()

	select {
	case <-s.ready:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %d cache gets; saw %d", s.targetGets, s.gets.Load())
	}
}

type gatedRoundTripper struct {
	calls      atomic.Int64
	statusCode int
	status     string
	body       []byte
	err        error
	release    chan struct{}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (rt *gatedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls.Add(1)
	select {
	case <-rt.release:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	if rt.err != nil {
		return nil, rt.err
	}
	return &http.Response{
		StatusCode: rt.statusCode,
		Status:     rt.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(rt.body)),
		Request:    req,
	}, nil
}
