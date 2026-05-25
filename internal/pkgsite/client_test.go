package pkgsite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/xcontext"
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

	got, err := client.Module(t.Context(), ModuleInput{
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

func TestClientModuleSchedulesPackagesWarm(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertPath(t, r, "/module/golang.org/x/oauth2")
		writeJSON(t, w, http.StatusOK, map[string]any{
			"path":    "golang.org/x/oauth2",
			"version": "v0.35.0",
		})
	}, WithWarmer(warmer))

	if _, err := client.Module(t.Context(), ModuleInput{ModulePath: "golang.org/x/oauth2"}); err != nil {
		t.Fatalf("Module returned error: %v", err)
	}

	assertWarmJobs(t, warmer.jobs(), []WarmJob{{
		Kind:     WarmPackages,
		Packages: PackagesInput{ModulePath: "golang.org/x/oauth2", Version: "v0.35.0"},
		Drain:    true,
	}})
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

	got, err := client.Symbols(t.Context(), SymbolsInput{
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

func TestClientPackageSchedulesSymbolsWarm(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertPath(t, r, "/package/golang.org/x/oauth2")
		writeJSON(t, w, http.StatusOK, map[string]any{
			"modulePath": "golang.org/x/oauth2",
			"version":    "v0.35.0",
		})
	}, WithWarmer(warmer))

	if _, err := client.Package(t.Context(), PackageInput{PackagePath: "golang.org/x/oauth2"}); err != nil {
		t.Fatalf("Package returned error: %v", err)
	}

	assertWarmJobs(t, warmer.jobs(), []WarmJob{{
		Kind:    WarmSymbols,
		Symbols: SymbolsInput{PackagePath: "golang.org/x/oauth2", ModulePath: "golang.org/x/oauth2", Version: "v0.35.0"},
		Drain:   true,
	}})
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

	got, err := client.Search(t.Context(), SearchInput{
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

func TestClientSearchSingleResultSchedulesPackageWarm(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertPath(t, r, "/search")
		writeJSON(t, w, http.StatusOK, map[string]any{
			"total": 1,
			"items": []map[string]any{{
				"path":       "github.com/google/uuid",
				"modulePath": "github.com/google/uuid",
				"version":    "v1.6.0",
			}},
		})
	}, WithWarmer(warmer))

	if _, err := client.Search(t.Context(), SearchInput{Query: "github.com/google/uuid"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	assertWarmJobs(t, warmer.jobs(), []WarmJob{{
		Kind:    WarmPackage,
		Package: PackageInput{PackagePath: "github.com/google/uuid", ModulePath: "github.com/google/uuid", Version: "v1.6.0"},
	}})
}

func TestClientSearchMultipleResultsDoesNotWarm(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		writeJSON(t, w, http.StatusOK, map[string]any{
			"total": 2,
			"items": []map[string]any{
				{"path": "example.com/one"},
				{"path": "example.com/two"},
			},
		})
	}, WithWarmer(warmer))

	if _, err := client.Search(t.Context(), SearchInput{Query: "example"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	assertWarmJobs(t, warmer.jobs(), nil)
}

func TestClientImportedByAcceptsStringItems(t *testing.T) {
	t.Parallel()

	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()

		assertPath(t, r, "/imported-by/slices")
		assertQuery(t, r, map[string][]string{
			"module": {"std"},
			"limit":  {"25"},
		})

		writeJSON(t, w, http.StatusOK, map[string]any{
			"modulePath": "std",
			"version":    "v1.26.3",
			"importedBy": map[string]any{
				"total": 2,
				"items": []string{"sort", "maps"},
			},
		})
	})

	got, err := client.ImportedBy(t.Context(), ImportedByInput{
		PackagePath: "slices",
		ModulePath:  "std",
	})
	if err != nil {
		t.Fatalf("ImportedBy returned error: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("ImportedBy returned Result.Error: %+v", got.Error)
	}
	assertSummary(t, got, map[string]any{
		"kind":        "imported_by",
		"packagePath": "slices",
		"modulePath":  "std",
		"version":     "v1.26.3",
		"count":       2,
	})
	assertItemPaths(t, got.Items, []string{"sort", "maps"})
	assertPagination(t, got, 2, 2, "")
}

func TestClientUpstream4xxReturnsStructuredResultError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     map[string]any
		callFunc func(*testing.T, *Client) (Result, error)
	}{
		{
			name:   "module not found",
			status: http.StatusNotFound,
			body: map[string]any{
				"code":    "not_found",
				"message": "module not found",
			},
			callFunc: func(t *testing.T, client *Client) (Result, error) {
				t.Helper()
				return client.Module(t.Context(), ModuleInput{ModulePath: "example.com/missing", Version: "v0.0.0"})
			},
		},
		{
			name:   "search bad request",
			status: http.StatusBadRequest,
			body: map[string]any{
				"code":    "bad_request",
				"message": "missing query",
			},
			callFunc: func(t *testing.T, client *Client) (Result, error) {
				t.Helper()
				return client.Search(t.Context(), SearchInput{Query: ""})
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

			got, err := tt.callFunc(t, client)
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

func TestClientErrorResultsDoNotWarm(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "missing"})
	}, WithWarmer(warmer))

	if _, err := client.Module(t.Context(), ModuleInput{ModulePath: "example.com/missing"}); err != nil {
		t.Fatalf("Module returned error: %v", err)
	}
	assertWarmJobs(t, warmer.jobs(), nil)
}

func TestAsyncWarmerDrainsPaginatedJobs(t *testing.T) {
	t.Parallel()

	tokens := make(chan string, 2)
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertPath(t, r, "/symbols/example.com/pkg")
		token := r.URL.Query().Get("token")
		tokens <- token
		payload := map[string]any{
			"modulePath": "example.com/pkg",
			"version":    "v1.0.0",
			"symbols": map[string]any{
				"total": 2,
				"items": []map[string]any{{"name": "A"}},
			},
		}
		if token == "" {
			payload["symbols"].(map[string]any)["nextPageToken"] = "page-2"
		}
		writeJSON(t, w, http.StatusOK, payload)
	})

	warmer := NewAsyncWarmer(client, AsyncWarmerOptions{Concurrency: 1, RequestTimeout: time.Second})
	warmer.Warm(t.Context(), WarmJob{
		Kind:    WarmSymbols,
		Symbols: SymbolsInput{PackagePath: "example.com/pkg", ModulePath: "example.com/pkg", Version: "v1.0.0"},
		Drain:   true,
	})

	got := collectStrings(t, tokens, 2)
	if !slices.Equal(got, []string{"", "page-2"}) {
		t.Fatalf("tokens = %#v, want %#v", got, []string{"", "page-2"})
	}
}

func TestAsyncWarmerSuppressesRecursiveWarming(t *testing.T) {
	t.Parallel()

	warmer := &recordingWarmer{}
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertPath(t, r, "/package/example.com/pkg")
		writeJSON(t, w, http.StatusOK, map[string]any{
			"modulePath": "example.com/pkg",
			"version":    "v1.0.0",
		})
	}, WithWarmer(warmer))

	async := NewAsyncWarmer(client, AsyncWarmerOptions{Concurrency: 1, RequestTimeout: time.Second})
	if outcome, err := async.run(xcontext.WithoutWarming(t.Context()), WarmJob{Kind: WarmPackage, Package: PackageInput{PackagePath: "example.com/pkg"}}); err != nil {
		t.Fatalf("warm run returned error: %v", err)
	} else if outcome != "success" {
		t.Fatalf("warm run outcome = %q, want success", outcome)
	}
	assertWarmJobs(t, warmer.jobs(), nil)
}

func TestAsyncWarmerHonorsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	client, _ := newFakeUpstreamClient(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		t.Helper()
		started <- r.URL.Path
		if r.URL.Path == "/package/example.com/one" {
			<-releaseFirst
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"modulePath": "example.com", "version": "v1.0.0"})
	})

	warmer := NewAsyncWarmer(client, AsyncWarmerOptions{Concurrency: 1, RequestTimeout: time.Second})
	warmer.Warm(t.Context(),
		WarmJob{Kind: WarmPackage, Package: PackageInput{PackagePath: "example.com/one"}},
		WarmJob{Kind: WarmPackage, Package: PackageInput{PackagePath: "example.com/two"}},
	)

	first := collectStrings(t, started, 1)
	if !slices.Equal(first, []string{"/package/example.com/one"}) {
		t.Fatalf("first started request = %#v, want first package", first)
	}
	assertNoStringWithin(t, started, 50*time.Millisecond)
	close(releaseFirst)
	second := collectStrings(t, started, 1)
	if !slices.Equal(second, []string{"/package/example.com/two"}) {
		t.Fatalf("second started request = %#v, want second package", second)
	}
}

func newFakeUpstreamClient(t *testing.T, handler func(*testing.T, http.ResponseWriter, *http.Request), opts ...Option) (*Client, string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(t, w, r)
	}))
	t.Cleanup(server.Close)

	client, err := New(config.Pkgsite{
		BaseURL:       server.URL,
		CacheDisabled: true,
	}, nil, opts...)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return client, server.URL
}

type recordingWarmer struct {
	mu       sync.Mutex
	recorded []WarmJob
}

func (w *recordingWarmer) Warm(_ context.Context, jobs ...WarmJob) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.recorded = append(w.recorded, jobs...)
}

func (w *recordingWarmer) jobs() []WarmJob {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]WarmJob(nil), w.recorded...)
}

func assertWarmJobs(t testing.TB, got, want []WarmJob) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warm jobs = %#v, want %#v", got, want)
	}
}

func collectStrings(t testing.TB, ch <-chan string, count int) []string {
	t.Helper()
	got := make([]string, 0, count)
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for len(got) < count {
		select {
		case value := <-ch:
			got = append(got, value)
		case <-timeout.C:
			t.Fatalf("timed out waiting for %d values, got %#v", count, got)
		}
	}
	return got
}

func assertNoStringWithin(t testing.TB, ch <-chan string, d time.Duration) {
	t.Helper()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case value := <-ch:
		t.Fatalf("unexpected value %q within %s", value, d)
	case <-timer.C:
	}
}

func assertHeader(t testing.TB, r *http.Request, key, want string) {
	t.Helper()

	if got := r.Header.Get(key); got != want {
		t.Errorf("%s header = %q, want %q", key, got, want)
	}
}

func assertPath(t testing.TB, r *http.Request, want string) {
	t.Helper()

	if got := r.URL.Path; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func assertQuery(t testing.TB, r *http.Request, want map[string][]string) {
	t.Helper()

	got := r.URL.Query()
	if len(got) != len(want) {
		t.Errorf("query = %#v, want %#v", got, want)
		return
	}
	for key, wantValues := range want {
		gotValues, ok := got[key]
		if !ok {
			t.Errorf("query missing key %q in %#v", key, got)
			continue
		}
		if !slices.Equal(gotValues, wantValues) {
			t.Errorf("query[%q] = %#v, want %#v", key, gotValues, wantValues)
		}
	}
}

func writeJSON(t testing.TB, w http.ResponseWriter, status int, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func assertSummary(t testing.TB, result Result, want map[string]any) {
	t.Helper()

	if !reflect.DeepEqual(result.Summary, want) {
		t.Fatalf("summary = %#v, want %#v", result.Summary, want)
	}
}

func assertItemNames(t testing.TB, items []map[string]any, want []string) {
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

func assertItemPaths(t testing.TB, items []map[string]any, want []string) {
	t.Helper()

	got := make([]string, 0, len(items))
	for _, item := range items {
		path, ok := item["path"].(string)
		if !ok {
			t.Fatalf("item path = %#v, want string", item["path"])
		}
		got = append(got, path)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("item paths = %#v, want %#v", got, want)
	}
}

func assertPagination(t testing.TB, result Result, total, displayed int, next string) {
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

func assertUpstreamURL(t testing.TB, got, want string) {
	t.Helper()

	gotURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse upstream URL %q: %v", got, err)
	}
	wantURL, err := url.Parse(want)
	if err != nil {
		t.Fatalf("parse wanted upstream URL %q: %v", want, err)
	}

	if gotURL.Scheme != wantURL.Scheme || gotURL.Host != wantURL.Host || gotURL.Path != wantURL.Path {
		t.Fatalf("upstream URL = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(gotURL.Query(), wantURL.Query()) {
		t.Fatalf("upstream URL query = %#v, want %#v", gotURL.Query(), wantURL.Query())
	}
}
