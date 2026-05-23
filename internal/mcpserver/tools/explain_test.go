package tools

import (
	"errors"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
)

func TestBuildExplainPayloadSummarizesSubResults(t *testing.T) {
	t.Parallel()

	payload := buildExplainPayload(pkgsite.ExplainInput{Path: "golang.org/x/oauth2", Version: "v1.2.3"}, explainParts{
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

	if payload.Summary.Kind != "module" {
		t.Fatalf("Kind = %q, want module", payload.Summary.Kind)
	}
	if payload.Summary.ModulePath != "golang.org/x/oauth2" {
		t.Fatalf("ModulePath = %q", payload.Summary.ModulePath)
	}
	if payload.Summary.ResolvedVersion != "v1.2.3" {
		t.Fatalf("ResolvedVersion = %q", payload.Summary.ResolvedVersion)
	}
	if payload.Summary.Counts["packages"] != 2 || payload.Summary.Counts["symbols"] != 3 || payload.Summary.Counts["vulns"] != 1 {
		t.Fatalf("Counts = %#v", payload.Summary.Counts)
	}
	if got := payload.Summary.KeySymbols; len(got) != 2 || got[0] != "Config" || got[1] != "Token" {
		t.Fatalf("KeySymbols = %#v", got)
	}
	if !payload.Summary.HasVulnerabilities {
		t.Fatal("HasVulnerabilities = false, want true")
	}
	if payload.SubResults.Package.Status != explainStatusNotFound {
		t.Fatalf("package status = %q", payload.SubResults.Package.Status)
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

func TestExplainSubResultDropsListRawPayloads(t *testing.T) {
	t.Parallel()

	list := explainSubResultFromResult(pkgsite.Result{
		Items:      []map[string]any{{"name": "Clone"}},
		Pagination: map[string]any{"total": 1},
		Raw:        map[string]any{"items": []any{map[string]any{"name": "Clone"}}},
	}, nil)
	if list.Result.Raw != nil {
		t.Fatalf("list raw = %#v, want nil", list.Result.Raw)
	}

	emptyList := explainSubResultFromResult(pkgsite.Result{
		Pagination: map[string]any{"total": 0},
		Raw:        map[string]any{"total": 0},
	}, nil)
	if emptyList.Result.Raw != nil {
		t.Fatalf("empty list raw = %#v, want nil", emptyList.Result.Raw)
	}

	single := explainSubResultFromResult(pkgsite.Result{
		Summary: map[string]any{"kind": "package"},
		Raw:     map[string]any{"path": "slices"},
	}, nil)
	if single.Result.Raw == nil {
		t.Fatal("single-object raw = nil, want preserved")
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
