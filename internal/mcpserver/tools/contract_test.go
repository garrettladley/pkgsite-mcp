package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolOutputContract(t *testing.T) {
	t.Parallel()
	session := newContractSession(t)
	listed, err := session.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 11 {
		t.Fatalf("listed %d tools, want 11", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if tool.OutputSchema == nil {
			t.Fatalf("%s has no outputSchema", tool.Name)
		}
	}
	tests := []struct {
		name   string
		args   map[string]any
		fields []string
	}{
		{toolNameModule, map[string]any{"module_path": "std", "include_readme": true, "include_licenses": true}, []string{"path", "version", "readme", "licenses", "commitTime"}},
		{toolNamePackage, map[string]any{"package_path": "net/http", "doc_format": "md", "include_imports": true, "include_licenses": true}, []string{"path", "version", "docs", "imports", "importCount", "licenses"}},
		{toolNameSearch, map[string]any{"query": "net/http"}, []string{"query", "items", "pagination"}},
		{toolNameVersions, map[string]any{"module_path": "std"}, []string{"modulePath", "items", "pagination"}},
		{toolNamePackages, map[string]any{"module_path": "std"}, []string{"modulePath", "version", "items", "pagination"}},
		{toolNameSymbols, map[string]any{"package_path": "net/http"}, []string{"packagePath", "modulePath", "version", "items", "pagination"}},
		{toolNameImportedBy, map[string]any{"package_path": "net/http"}, []string{"packagePath", "items", "pagination"}},
		{toolNameVulns, map[string]any{"path": "net/http"}, []string{"path", "items", "pagination"}},
		{toolNameExplain, map[string]any{"path": "net/http"}, []string{"summary", "lookups"}},
		{toolNameListSkills, map[string]any{}, []string{"items", "pagination"}},
		{toolNameLoadSkill, map[string]any{"skill_name": "pkgsite/overview", "header_only": true}, []string{"name", "description", "related", "resources"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, data := callContractTool(t, session, tt.name, tt.args)
			if result.IsError {
				t.Fatalf("tool error: %+v", data)
			}
			assertJSONFallback(t, result, data)
			for _, field := range append(tt.fields, "source") {
				if _, ok := data[field]; !ok {
					t.Fatalf("missing %q: %+v", field, data)
				}
			}
			for _, field := range []string{"raw", "fromCache", "subResults"} {
				if _, ok := data[field]; ok {
					t.Fatalf("unexpected %q: %+v", field, data)
				}
			}
			if tt.name != toolNameExplain {
				if _, ok := data["summary"]; ok {
					t.Fatalf("duplicate summary: %+v", data)
				}
			}
		})
	}
}

func TestToolErrorsAndEmptyResults(t *testing.T) {
	t.Parallel()
	session := newContractSession(t)
	t.Run("actionable api failure", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameModule, map[string]any{"module_path": "net/http"})
		if !result.IsError {
			t.Fatal("API failure did not set isError")
		}
		assertJSONFallback(t, result, data)
		apiErr := data["error"].(map[string]any)
		if apiErr["code"] != float64(400) || len(apiErr["fixes"].([]any)) == 0 {
			t.Fatalf("error = %+v", apiErr)
		}
		if _, ok := apiErr["body"]; ok {
			t.Fatal("duplicated upstream error body")
		}
	})
	t.Run("response decoding error", func(t *testing.T) {
		t.Parallel()
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolNamePackage, Arguments: map[string]any{"package_path": "bad-json"}})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Fatal("response decoding failure did not set isError")
		}
	})
	t.Run("unresolved explain", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameExplain, map[string]any{"path": "missing"})
		if !result.IsError || data["summary"].(map[string]any)["kind"] != "unknown" {
			t.Fatalf("result = %+v", data)
		}
		assertJSONFallback(t, result, data)
	})
	t.Run("upstream total preserved", func(t *testing.T) {
		t.Parallel()
		_, data := callContractTool(t, session, toolNameSymbols, map[string]any{"package_path": "net/http"})
		page := data["pagination"].(map[string]any)
		if page["total"] != float64(406) || page["fetchedItems"] != float64(1) || page["nextStartAt"] != nil || page["upstreamNextPageToken"] != "symbols-next" {
			t.Fatalf("pagination = %+v", page)
		}
	})
	t.Run("retry after", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameSymbols, map[string]any{"package_path": "rate-limited"})
		if !result.IsError || data["error"].(map[string]any)["retryAfter"] != "2" {
			t.Fatalf("result = %+v", data)
		}
		assertJSONFallback(t, result, data)
	})
	t.Run("empty items are an array", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameVulns, map[string]any{"path": "net/http"})
		if result.IsError {
			t.Fatalf("result = %+v", data)
		}
		items, ok := data["items"].([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("items = %#v", data["items"])
		}
	})
	t.Run("imports omitted", func(t *testing.T) {
		t.Parallel()
		_, data := callContractTool(t, session, toolNamePackage, map[string]any{"package_path": "net/http"})
		if _, ok := data["importCount"]; ok {
			t.Fatal("importCount without requested imports")
		}
	})
	t.Run("handler validation", func(t *testing.T) {
		t.Parallel()
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolNamePackage, Arguments: map[string]any{"package_path": " "}})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "package_path is required") {
			t.Fatalf("result = %+v", result)
		}
	})
	t.Run("partial explain", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameExplain, map[string]any{"path": "net/http"})
		if result.IsError {
			t.Fatal("partial explain should succeed")
		}
		lookups := data["lookups"].(map[string]any)
		if lookups["module"].(map[string]any)["status"] != "error" || lookups["package"].(map[string]any)["status"] != "ok" {
			t.Fatalf("lookups = %+v", lookups)
		}
		if _, ok := lookups["module"].(map[string]any)["data"]; ok {
			t.Fatal("failed lookup has success data")
		}
	})
	t.Run("markdown guide with structured header", func(t *testing.T) {
		t.Parallel()
		result, data := callContractTool(t, session, toolNameLoadSkill, map[string]any{"skill_name": "pkgsite/overview"})
		if result.IsError || data["name"] != "pkgsite/overview" {
			t.Fatalf("result = %+v", data)
		}
		if !strings.HasPrefix(result.Content[0].(*mcp.TextContent).Text, "# pkgsite-mcp overview") {
			t.Fatal("guide is not Markdown")
		}
	})
}

func callContractTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	data, err := decodeOutput[map[string]any](result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatalf("missing structuredContent: %+v", result)
	}
	return result, data
}

func assertJSONFallback(t *testing.T, result *mcp.CallToolResult, data map[string]any) {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content count = %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T", result.Content[0])
	}
	var fallback map[string]any
	if err := json.Unmarshal([]byte(text.Text), &fallback); err != nil {
		t.Fatalf("text is not plain JSON: %v", err)
	}
	if !reflect.DeepEqual(data, fallback) {
		t.Fatalf("text and structuredContent differ: %+v / %+v", fallback, data)
	}
}

func newContractSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fixtures := map[string]string{
			"/module/std":           `{"path":"std","version":"v1.27.2","commitTime":"2026-10-08T15:22:57Z","isStandardLibrary":true,"readme":{"contents":"readme"},"licenses":[{"types":["BSD-3-Clause"]}]}`,
			"/package/net/http":     `{"path":"net/http","name":"http","modulePath":"std","version":"v1.27.2","isStandardLibrary":true}`,
			"/search":               `{"items":[{"packagePath":"net/http","modulePath":"std","version":"v1.27.2"}],"nextPageToken":"search-next","total":12}`,
			"/versions/std":         `{"items":[{"modulePath":"std","version":"v1.27.2","commitTime":"2026-10-08T15:22:57Z","deprecated":false}],"total":20}`,
			"/packages/std":         `{"modulePath":"std","version":"v1.27.2","isStandardLibrary":true,"packages":{"items":[{"path":"net/http","name":"http"}],"total":400}}`,
			"/symbols/net/http":     `{"modulePath":"std","version":"v1.27.2","symbols":{"items":[{"name":"Client","kind":"Type","parent":"Client","synopsis":"type Client struct{ ... }"}],"total":406,"nextPageToken":"symbols-next"}}`,
			"/imported-by/net/http": `{"modulePath":"std","version":"v1.27.2","importedBy":{"items":["example.com/client"],"total":1000,"nextPageToken":"importers-next"}}`,
			"/package/bad-json":     `not JSON`,
			"/vulns/net/http":       `{"items":[],"total":0}`,
		}
		if r.URL.Path == "/symbols/rate-limited" {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"code":429,"message":"try again later"}`)); err != nil {
				t.Error(err)
			}
			return
		}
		body, ok := fixtures[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			body = `{"code":400,"message":"net/http is a package, not a module","fixes":["retry with std"]}`
		}
		if r.URL.Path == "/package/net/http" {
			writePackageFixture(t, w, r, body)
			return
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(upstream.Close)
	client, err := pkgsite.New(config.Pkgsite{BaseURL: upstream.URL, CacheDisabled: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	Register(server, client)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := serverSession.Close(); err != nil {
			t.Error(err)
		}
	})
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	return session
}

func writePackageFixture(t *testing.T, w http.ResponseWriter, r *http.Request, body string) {
	t.Helper()
	var pkg map[string]any
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		t.Error(err)
		return
	}
	if r.URL.Query().Get("imports") == "true" {
		pkg["imports"] = []string{"io", "net"}
	}
	if r.URL.Query().Get("doc") != "" {
		pkg["docs"] = "# Package http"
	}
	if r.URL.Query().Get("licenses") == "true" {
		pkg["licenses"] = []map[string]any{{"types": []string{"BSD-3-Clause"}}}
	}
	if err := json.NewEncoder(w).Encode(pkg); err != nil {
		t.Error(err)
	}
}
