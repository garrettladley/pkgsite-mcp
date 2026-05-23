package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPRequestMetadataExtractsToolCall(t *testing.T) {
	t.Parallel()

	const body = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pkgsite_search","arguments":{"query":"slices"}}}`
	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get(HeaderInternalMCPMethod), "tools/call"; got != want {
			t.Fatalf("method = %q, want %q", got, want)
		}
		if got, want := r.Header.Get(HeaderInternalMCPName), "pkgsite_search"; got != want {
			t.Fatalf("name = %q, want %q", got, want)
		}
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBody) != body {
			t.Fatalf("body = %q, want %q", string(gotBody), body)
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMCPRequestMetadataExtractsListMethodWithoutName(t *testing.T) {
	t.Parallel()

	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get(HeaderInternalMCPMethod), "tools/list"; got != want {
			t.Fatalf("method = %q, want %q", got, want)
		}
		if got := r.Header.Get(HeaderInternalMCPName); got != "" {
			t.Fatalf("name = %q, want empty", got)
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMCPRequestMetadataSkipsNonMCPRequest(t *testing.T) {
	t.Parallel()

	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(HeaderInternalMCPMethod); got != "" {
			t.Fatalf("method = %q, want empty", got)
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/other", strings.NewReader(`{"method":"tools/list"}`))
	handler.ServeHTTP(httptest.NewRecorder(), req)
}
