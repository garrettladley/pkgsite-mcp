package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
)

func TestBuildExplainPayloadSummarizesSubResults(t *testing.T) {
	t.Parallel()

	summary := buildExplainSummary(pkgsite.ExplainInput{Path: "golang.org/x/oauth2", Version: "v1.2.3"}, explainParts{
		Module: explainSubResultFromResult(pkgsite.Result{Summary: map[string]any{
			"kind": "module", "path": "golang.org/x/oauth2", "version": "v1.2.3", "isLatest": true,
		}, Raw: map[string]any{"module": true}}, nil),
		Package: explainSubResultFromResult(pkgsite.Result{Error: &pkgsite.APIError{StatusCode: 404, Message: "package not found"}}, nil),
		Packages: explainSubResultFromResult(pkgsite.Result{
			Summary: map[string]any{"kind": "module_packages", "modulePath": "golang.org/x/oauth2", "version": "v1.2.3"},
			Items: []map[string]any{
				{"path": "golang.org/x/oauth2"},
				{"path": "golang.org/x/oauth2/google"},
			},
		}, nil),
		Symbols: explainSubResultFromResult(pkgsite.Result{
			Summary: map[string]any{"kind": "symbols", "modulePath": "golang.org/x/oauth2", "version": "v1.2.3"},
			Items: []map[string]any{
				{"name": "Config"},
				{"Name": "Token"},
				{"name": "Config"},
			},
		}, nil),
		Vulns: explainSubResultFromResult(pkgsite.Result{Items: []map[string]any{{"id": "GO-2026-0001"}}}, nil),
	})

	if summary.Kind != "module" {
		t.Fatalf("Kind = %q, want module", summary.Kind)
	}
	if summary.ModulePath != "golang.org/x/oauth2" {
		t.Fatalf("ModulePath = %q", summary.ModulePath)
	}
	if summary.ResolvedVersion != "v1.2.3" {
		t.Fatalf("ResolvedVersion = %q", summary.ResolvedVersion)
	}
	if summary.Counts["packages"] != 2 || summary.Counts["symbols"] != 3 || summary.Counts["vulns"] != 1 {
		t.Fatalf("Counts = %#v", summary.Counts)
	}
	if got := summary.KeySymbols; len(got) != 2 || got[0] != "Config" || got[1] != "Token" {
		t.Fatalf("KeySymbols = %#v", got)
	}
	if !summary.HasVulnerabilities {
		t.Fatal("HasVulnerabilities = false, want true")
	}
}

func TestBuildExplainPayloadExposesUpstreamMetadata(t *testing.T) {
	t.Parallel()

	summary := buildExplainSummary(pkgsite.ExplainInput{Path: "example.com/module/pkg"}, explainParts{
		Module: explainSubResultFromResult(pkgsite.Result{Summary: map[string]any{
			"kind": "module", "path": "example.com/module", "version": "v1.2.3", "repoUrl": "https://example.com/module", "hasGoMod": true, "isRedistributable": true,
		}}, nil),
		Package: explainSubResultFromResult(pkgsite.Result{Summary: map[string]any{
			"kind": "package", "path": "example.com/module/pkg", "modulePath": "example.com/module", "version": "v1.2.3", "name": "pkg", "synopsis": "Package synopsis.", "goos": "linux", "goarch": "amd64", "importCount": 3, "isLatest": true, "isRedistributable": true,
		}}, nil),
	})

	if summary.RepoURL != "https://example.com/module" || !summary.HasGoMod || !summary.IsRedistributable {
		t.Fatalf("module metadata missing from summary: %#v", summary)
	}
	if summary.Name != "pkg" || summary.Synopsis != "Package synopsis." || summary.Goos != "linux" || summary.Goarch != "amd64" {
		t.Fatalf("package metadata missing from summary: %#v", summary)
	}
}

func TestExplainSubResultFromResultPreservesCallErrorsAndAPIResults(t *testing.T) {
	t.Parallel()

	callErr := explainSubResultFromResult(pkgsite.Result{Summary: map[string]any{"ignored": true}}, errors.New("network down"))
	if callErr.Status != explainStatusError {
		t.Fatalf("callErr.Status = %q", callErr.Status)
	}
	if callErr.Result.Summary != nil {
		t.Fatalf("call error should not preserve partial result: %#v", callErr.Result)
	}

	apiErr := explainSubResultFromResult(pkgsite.Result{Error: &pkgsite.APIError{StatusCode: 500, Message: "upstream failed"}}, nil)
	if apiErr.Status != explainStatusError || apiErr.Result.Error == nil {
		t.Fatalf("apiErr = %#v", apiErr)
	}
}

func TestExplainLookupsHaveTypedDataAndActionableErrors(t *testing.T) {
	t.Parallel()
	parts := explainParts{
		Module:   explainSubResultFromResult(pkgsite.Result{Error: &pkgsite.APIError{StatusCode: 400, Message: "use std", Fixes: []string{"module_path=std"}}}, nil),
		Package:  explainSubResultFromResult(pkgsite.Result{Raw: map[string]any{"path": "net/http", "modulePath": "std"}}, nil),
		Packages: explainSubResultSkipped("package lookup"),
		Symbols:  explainSubResultFromResult(pkgsite.Result{Items: []map[string]any{{"name": "Client", "kind": "Type"}}}, nil),
		Vulns:    explainSubResultFromResult(pkgsite.Result{}, nil),
	}
	payload, err := buildExplainPayload(t.Context(), pkgsite.ExplainInput{Path: "net/http"}, parts)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Lookups.Module.Data != nil || payload.Lookups.Module.Error == nil || len(payload.Lookups.Module.Error.Fixes) != 1 {
		t.Fatalf("module lookup = %+v", payload.Lookups.Module)
	}
	if payload.Lookups.Package.Data == nil || *payload.Lookups.Package.Data.Path != "net/http" {
		t.Fatalf("package lookup = %+v", payload.Lookups.Package)
	}
	if payload.Lookups.Packages.Data != nil || payload.Lookups.Packages.Reason == "" {
		t.Fatalf("packages lookup = %+v", payload.Lookups.Packages)
	}
	if payload.Lookups.Symbols.Data == nil || len(payload.Lookups.Symbols.Data.Items) != 1 {
		t.Fatalf("symbols lookup = %+v", payload.Lookups.Symbols)
	}
}

func TestLooksModuleLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "github.com/garrettladley/pkgsite-mcp", want: true},
		{path: "github.com/garrettladley/pkgsite-mcp/v2", want: true},
		{path: "github.com/garrettladley/pkgsite-mcp/internal/mcpserver", want: false},
		{path: "golang.org/x/oauth2/google", want: false},
		{path: "std", want: true},
		{path: "net/http", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := looksModuleLike(tt.path); got != tt.want {
				t.Fatalf("looksModuleLike(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestExplainBudgetsCombinedResponse(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"name": "Client", "synopsis": strings.Repeat("a", 500)},
		{"name": "Client.Do", "synopsis": strings.Repeat("b", 500)},
		{"name": "Client.Get", "synopsis": strings.Repeat("c", 500)},
	}
	parts := explainParts{
		Module:   explainSubResultSkipped("package path"),
		Package:  explainSubResultFromResult(pkgsite.Result{Raw: map[string]any{"path": "net/http", "modulePath": "std"}}, nil),
		Packages: explainSubResultSkipped("package path"),
		Symbols:  explainSubResultFromResult(pkgsite.Result{Items: items, Pagination: map[string]any{"total": 406, "upstreamNextPageToken": "next"}}, nil),
		Vulns:    explainSubResultFromResult(pkgsite.Result{}, nil),
	}
	full, err := buildExplainPayload(t.Context(), pkgsite.ExplainInput{Path: "net/http"}, parts)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	// Remove roughly one item from the overall budget, including all lookup overhead.
	budget := (len(data) - 400) / 4
	output, err := buildExplainPayload(t.Context(), pkgsite.ExplainInput{Path: "net/http", PageInput: pkgsite.PageInput{MaxTokens: budget}}, parts)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > budget*4 {
		t.Fatalf("explain exceeds budget: %d > %d", len(data), budget*4)
	}
	symbols := output.Lookups.Symbols.Data
	if len(symbols.Items) != 2 || symbols.Pagination.NextStartAt == nil || *symbols.Pagination.NextStartAt != 2 || *symbols.Pagination.Total != 406 {
		t.Fatalf("symbols = %+v", symbols)
	}
	if _, err := buildExplainPayload(t.Context(), pkgsite.ExplainInput{Path: "net/http", PageInput: pkgsite.PageInput{MaxTokens: 1}}, parts); err == nil {
		t.Fatal("tiny explain budget should report an error")
	}
}
