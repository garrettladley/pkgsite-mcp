package observability

import (
	"net/url"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
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

func TestInitializeAttrs(t *testing.T) {
	t.Parallel()

	attrs := InitializeAttrs{
		ClientName:            " codex-mcp-client ",
		ClientTitle:           "Codex",
		ClientVersion:         "1.2.3",
		ProtocolVersion:       "2025-06-18",
		ProtocolVersionHeader: "2025-06-18",
	}.Attributes()

	assertStringAttr(t, attrs, AttrMCPClientName, "codex-mcp-client")
	assertStringAttr(t, attrs, AttrMCPClientTitle, "Codex")
	assertStringAttr(t, attrs, AttrMCPClientVersion, "1.2.3")
	assertStringAttr(t, attrs, AttrMCPProtocolVersion, "2025-06-18")
	assertStringAttr(t, attrs, AttrMCPProtocolVersionHeader, "2025-06-18")
}

func TestInitializeAttrsDefaultsMissingClientInfo(t *testing.T) {
	t.Parallel()

	attrs := InitializeAttrs{}.Attributes()

	assertStringAttr(t, attrs, AttrMCPClientName, "unknown")
	assertStringAttr(t, attrs, AttrMCPClientTitle, "unknown")
	assertStringAttr(t, attrs, AttrMCPClientVersion, "unknown")
	assertStringAttr(t, attrs, AttrMCPProtocolVersion, "unknown")
	if slices.ContainsFunc(attrs, func(attr attribute.KeyValue) bool {
		return string(attr.Key) == AttrMCPProtocolVersionHeader
	}) {
		t.Fatalf("protocol header attr present, want omitted")
	}
}

func assertStringAttr(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()

	for _, attr := range attrs {
		if string(attr.Key) == key {
			if got := attr.Value.AsString(); got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s missing from attrs", key)
}
