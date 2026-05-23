package observability

import (
	"net/url"
	"testing"
)

func TestEndpointFromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want PkgsiteEndpoint
	}{
		{raw: "https://pkg.go.dev/v1beta/search?q=uuid", want: PkgsiteEndpointSearch},
		{raw: "https://pkg.go.dev/v1beta/module/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointModule},
		{raw: "https://pkg.go.dev/v1beta/package/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointPackage},
		{raw: "https://pkg.go.dev/v1beta/versions/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointVersions},
		{raw: "https://pkg.go.dev/v1beta/packages/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointPackages},
		{raw: "https://pkg.go.dev/v1beta/symbols/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointSymbols},
		{raw: "https://pkg.go.dev/v1beta/imported-by/golang.org%2Fx%2Foauth2", want: PkgsiteEndpointImportedBy},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := EndpointFromURL(u); got != tt.want {
				t.Fatalf("EndpointFromURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    VersionClass
	}{
		{version: "", want: VersionClassEmpty},
		{version: "latest", want: VersionClassFloating},
		{version: "main", want: VersionClassBranch},
		{version: "master", want: VersionClassBranch},
		{version: "v1.2.3", want: VersionClassPinned},
		{version: "abcdef", want: VersionClassOther},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			t.Parallel()

			if got := ClassifyVersion(tt.version); got != tt.want {
				t.Fatalf("ClassifyVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestRemainingBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		remaining int64
		limit     int
		want      string
	}{
		{remaining: 0, limit: 100, want: "empty"},
		{remaining: 20, limit: 100, want: "low"},
		{remaining: 30, limit: 100, want: "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := RemainingBucket(tt.remaining, tt.limit); got != tt.want {
				t.Fatalf("RemainingBucket(%d, %d) = %q, want %q", tt.remaining, tt.limit, got, tt.want)
			}
		})
	}
}
