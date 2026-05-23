package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/middleware"
)

func TestHealthDoesNotExposeBuildMetadata(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	health(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json"; got != want {
		t.Fatalf("content type = %q, want %q", got, want)
	}
}

func TestHTTPSpanNameIncludesMCPMethodAndName(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	req.Header.Set(middleware.HeaderInternalMCPMethod, "tools/call")
	req.Header.Set(middleware.HeaderInternalMCPName, "pkgsite_search")

	if got, want := httpSpanName("", req), "POST /mcp tools/call pkgsite_search"; got != want {
		t.Fatalf("httpSpanName() = %q, want %q", got, want)
	}
}

func TestHTTPSpanNameIncludesMCPMethodWithoutName(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	req.Header.Set(middleware.HeaderInternalMCPMethod, "tools/list")

	if got, want := httpSpanName("", req), "POST /mcp tools/list"; got != want {
		t.Fatalf("httpSpanName() = %q, want %q", got, want)
	}
}

func TestHTTPSpanNameFallsBackToHTTPRoute(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	req.Pattern = "GET /health"

	if got, want := httpSpanName("", req), "GET /health"; got != want {
		t.Fatalf("httpSpanName() = %q, want %q", got, want)
	}
}
