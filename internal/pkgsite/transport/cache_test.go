package transport

import (
	"net/http"
	"net/url"
	"testing"
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
