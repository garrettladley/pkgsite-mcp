package middleware

import (
	"errors"
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

func TestMCPRequestMetadataClearsCallerSuppliedInternalHeaders(t *testing.T) {
	t.Parallel()

	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(HeaderInternalMCPMethod); got != "" {
			t.Fatalf("method = %q, want empty", got)
		}
		if got := r.Header.Get(HeaderInternalMCPName); got != "" {
			t.Fatalf("name = %q, want empty", got)
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/other", strings.NewReader(`{"method":"tools/list"}`))
	req.Header.Set(HeaderInternalMCPMethod, "tools/call")
	req.Header.Set(HeaderInternalMCPName, "pkgsite_search")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMCPRequestMetadataSkipsUnknownLengthBodyWithoutTruncating(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("x", int(maxMCPMetadataBodyBytes)+1)
	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(HeaderInternalMCPMethod); got != "" {
			t.Fatalf("method = %q, want empty", got)
		}
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBody) != body {
			t.Fatalf("body length = %d, want %d", len(gotBody), len(body))
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.ContentLength = -1
	req.Header.Set(HeaderInternalMCPMethod, "tools/list")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMCPRequestMetadataRestoresPartialBodyAfterReadError(t *testing.T) {
	t.Parallel()

	const body = `{"jsonrpc":"2.0"`
	handler := MCPRequestMetadata(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(HeaderInternalMCPMethod); got != "" {
			t.Fatalf("method = %q, want empty", got)
		}
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBody) != body {
			t.Fatalf("body = %q, want %q", string(gotBody), body)
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", &errorReader{data: body})
	req.ContentLength = int64(len(body))
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

type errorReader struct {
	data string
	done bool
}

func (r *errorReader) Close() error {
	return nil
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), errors.New("read failed")
}
