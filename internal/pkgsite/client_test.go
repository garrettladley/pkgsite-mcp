package pkgsite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
)

func TestClientModuleSuccessFromFakeUpstream(t *testing.T) {
	t.Parallel()

	client, upstreamURL := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()

		assertHeader(t, r, "Accept", "application/json")
		assertPath(t, r, "/module/golang.org/x/oauth2")
		assertQuery(t, r, map[string][]string{
			"version": {"v0.35.0"},
			"readme":  {"true"},
		})

		writeJSON(t, w, http.StatusOK, map[string]any{
			"path":              "golang.org/x/oauth2",
			"version":           "v0.35.0",
			"isLatest":          true,
			"repoUrl":           "https://go.googlesource.com/oauth2",
			"hasGoMod":          true,
			"isRedistributable": true,
			"isStandardLibrary": false,
		})
	})

	got, err := client.Module(context.Background(), ModuleInput{
		ModulePath:    "golang.org/x/oauth2",
		Version:       "v0.35.0",
		IncludeReadme: true,
	})
	if err != nil {
		t.Fatalf("Module returned error: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("Module returned Result.Error: %+v", got.Error)
	}
	assertSummary(t, got, map[string]any{
		"kind":              "module",
		"path":              "golang.org/x/oauth2",
		"version":           "v0.35.0",
		"isLatest":          true,
		"latest":            true,
		"repoUrl":           "https://go.googlesource.com/oauth2",
		"hasGoMod":          true,
		"isRedistributable": true,
		"isStandardLibrary": false,
	})
	assertUpstreamURL(t, got.UpstreamURL, upstreamURL+"/module/golang.org%2Fx%2Foauth2?readme=true&version=v0.35.0")
}

func TestClientSymbolsSuccessFromFakeUpstream(t *testing.T) {
	t.Parallel()

	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()

		assertPath(t, r, "/symbols/golang.org/x/oauth2")
		assertQuery(t, r, map[string][]string{
			"module":  {"golang.org/x/oauth2"},
			"version": {"v0.35.0"},
			"limit":   {"10"},
			"token":   {"next-symbol-page"},
		})

		writeJSON(t, w, http.StatusOK, map[string]any{
			"modulePath": "golang.org/x/oauth2",
			"version":    "v0.35.0",
			"symbols": map[string]any{
				"total":         4,
				"nextPageToken": "after-token",
				"items": []map[string]any{
					{"name": "TokenSource", "kind": "type"},
					{"name": "Config", "kind": "type"},
					{"name": "Token", "kind": "type"},
					{"name": "NewClient", "kind": "func"},
				},
			},
		})
	})

	got, err := client.Symbols(context.Background(), SymbolsInput{
		PackagePath: "golang.org/x/oauth2",
		ModulePath:  "golang.org/x/oauth2",
		Version:     "v0.35.0",
		Limit:       10,
		Token:       "next-symbol-page",
	})
	if err != nil {
		t.Fatalf("Symbols returned error: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("Symbols returned Result.Error: %+v", got.Error)
	}
	assertSummary(t, got, map[string]any{
		"kind":        "symbols",
		"packagePath": "golang.org/x/oauth2",
		"modulePath":  "golang.org/x/oauth2",
		"version":     "v0.35.0",
		"count":       4,
	})
	assertItemNames(t, got.Items, []string{"TokenSource", "Config", "Token", "NewClient"})
	assertPagination(t, got, 4, 4, "after-token")
}

func TestClientSearchSuccessFromFakeUpstream(t *testing.T) {
	t.Parallel()

	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()

		assertPath(t, r, "/search")
		assertQuery(t, r, map[string][]string{
			"q":      {"github.com/google/uuid"},
			"limit":  {"2"},
			"filter": {"uuid"},
		})

		writeJSON(t, w, http.StatusOK, map[string]any{
			"total": 1,
			"items": []map[string]any{
				{
					"name":       "uuid",
					"path":       "github.com/google/uuid",
					"modulePath": "github.com/google/uuid",
					"version":    "v1.6.0",
				},
			},
		})
	})

	got, err := client.Search(context.Background(), SearchInput{
		Query:  "github.com/google/uuid",
		Limit:  2,
		Filter: "uuid",
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("Search returned Result.Error: %+v", got.Error)
	}
	assertSummary(t, got, map[string]any{
		"query":  "github.com/google/uuid",
		"symbol": "",
		"count":  1,
	})
	assertItemNames(t, got.Items, []string{"uuid"})
	assertPagination(t, got, 1, 1, "")
}

func TestClientUpstream4xxReturnsStructuredResultError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     map[string]any
		callFunc func(context.Context, *Client) (Result, error)
	}{
		{
			name:   "module not found",
			status: http.StatusNotFound,
			body: map[string]any{
				"code":    "not_found",
				"message": "module not found",
			},
			callFunc: func(ctx context.Context, client *Client) (Result, error) {
				return client.Module(ctx, ModuleInput{ModulePath: "example.com/missing", Version: "v0.0.0"})
			},
		},
		{
			name:   "search bad request",
			status: http.StatusBadRequest,
			body: map[string]any{
				"code":    "bad_request",
				"message": "missing query",
			},
			callFunc: func(ctx context.Context, client *Client) (Result, error) {
				return client.Search(ctx, SearchInput{Query: ""})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, upstreamURL := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				t.Helper()
				writeJSON(t, w, tt.status, tt.body)
			})

			got, err := tt.callFunc(context.Background(), client)
			if err != nil {
				t.Fatalf("client call returned error: %v", err)
			}
			if got.Error == nil {
				t.Fatal("Result.Error is nil")
			}
			if got.Error.StatusCode != tt.status {
				t.Fatalf("status code = %d, want %d", got.Error.StatusCode, tt.status)
			}
			wantStatus := fmt.Sprintf("%d %s", tt.status, http.StatusText(tt.status))
			if got.Error.Status != wantStatus {
				t.Fatalf("status = %q, want %q", got.Error.Status, wantStatus)
			}
			if !json.Valid(got.Error.Body) {
				t.Fatalf("Result.Error.Body is not valid JSON: %q", string(got.Error.Body))
			}
			for _, want := range []string{tt.body["code"].(string), tt.body["message"].(string)} {
				if !strings.Contains(got.Error.Message, want) {
					t.Fatalf("message %q does not contain %q", got.Error.Message, want)
				}
			}
			if !strings.HasPrefix(got.UpstreamURL, upstreamURL+"/") {
				t.Fatalf("upstream URL = %q, want prefix %q", got.UpstreamURL, upstreamURL+"/")
			}
		})
	}
}

func newFakeUpstreamClient(t *testing.T, handler func(*testing.T, http.ResponseWriter, *http.Request)) (*Client, string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(t, w, r)
	}))
	t.Cleanup(server.Close)

	client, err := New(config.Pkgsite{
		BaseURL:       server.URL,
		CacheDisabled: true,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return client, server.URL
}

func assertHeader(t *testing.T, r *http.Request, key, want string) {
	t.Helper()

	if got := r.Header.Get(key); got != want {
		t.Fatalf("%s header = %q, want %q", key, got, want)
	}
}

func assertPath(t *testing.T, r *http.Request, want string) {
	t.Helper()

	if got := r.URL.Path; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func assertQuery(t *testing.T, r *http.Request, want map[string][]string) {
	t.Helper()

	got := r.URL.Query()
	if len(got) != len(want) {
		t.Fatalf("query = %#v, want %#v", got, want)
	}
	for key, wantValues := range want {
		gotValues, ok := got[key]
		if !ok {
			t.Fatalf("query missing key %q in %#v", key, got)
		}
		if !slices.Equal(gotValues, wantValues) {
			t.Fatalf("query[%q] = %#v, want %#v", key, gotValues, wantValues)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func assertSummary(t *testing.T, result Result, want map[string]any) {
	t.Helper()

	if !reflect.DeepEqual(result.Summary, want) {
		t.Fatalf("summary = %#v, want %#v", result.Summary, want)
	}
}

func assertItemNames(t *testing.T, items []map[string]any, want []string) {
	t.Helper()

	got := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item["name"].(string)
		if !ok {
			t.Fatalf("item name = %#v, want string", item["name"])
		}
		got = append(got, name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("item names = %#v, want %#v", got, want)
	}
}

func assertPagination(t *testing.T, result Result, total, displayed int, next string) {
	t.Helper()

	want := map[string]any{
		"total":                 total,
		"displayedItems":        displayed,
		"startAt":               0,
		"nextStartAt":           nil,
		"upstreamNextPageToken": next,
	}
	if !reflect.DeepEqual(result.Pagination, want) {
		t.Fatalf("pagination = %#v, want %#v", result.Pagination, want)
	}
}

func assertUpstreamURL(t *testing.T, got, want string) {
	t.Helper()

	if got != want {
		t.Fatalf("upstream URL = %q, want %q", got, want)
	}
}
