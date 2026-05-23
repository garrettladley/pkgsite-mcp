package tools

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
)

func TestPaginatedEnvelopeMatchesGoldenShape(t *testing.T) {
	t.Parallel()

	got := paginatedEnvelope(pkgsite.Result{
		Summary: map[string]any{
			"count":       2,
			"kind":        "symbols",
			"packagePath": "golang.org/x/oauth2",
		},
		Items: []map[string]any{
			{"name": "Config"},
			{"name": "Token"},
		},
		Pagination: map[string]any{
			"upstreamNextPageToken": "upstream-token",
		},
		Raw: map[string]any{
			"nextPageToken": "upstream-token",
		},
		UpstreamURL: "https://pkg.go.dev/v1beta/symbols/golang.org/x/oauth2?version=v0.35.0",
		FromCache:   true,
	}, pkgsite.PageInput{}, envelopeOptions{
		Source:      "pkg.go.dev/v1beta",
		UpstreamURL: "https://pkg.go.dev/v1beta/symbols/golang.org/x/oauth2?version=v0.35.0",
		ToolName:    toolNameSymbols,
		NextArgs: map[string]any{
			"package_path": "golang.org/x/oauth2",
			"version":      "v0.35.0",
		},
	})

	want := `<METADATA>
  <is_truncated>false</is_truncated>
  <displayed_items>2</displayed_items>
  <count>2</count>
  <start_at>0</start_at>
  <source>pkg.go.dev/v1beta</source>
  <upstream_url>https://pkg.go.dev/v1beta/symbols/golang.org/x/oauth2?version=v0.35.0</upstream_url>
</METADATA>
<JSON_DATA>
{
  "summary": {
    "count": 2,
    "kind": "symbols",
    "packagePath": "golang.org/x/oauth2"
  },
  "items": [
    {
      "name": "Config"
    },
    {
      "name": "Token"
    }
  ],
  "pagination": {
    "displayedItems": 2,
    "nextStartAt": null,
    "startAt": 0,
    "total": 2,
    "upstreamNextPageToken": "upstream-token"
  }
}
</JSON_DATA>`

	if got != want {
		t.Fatalf("envelope mismatch (-want +got):\n%s", diffStrings(want, got))
	}
	if strings.Contains(got, `"raw"`) {
		t.Fatalf("paginated envelope should not contain raw payload:\n%s", got)
	}
}

func TestSingleEnvelopeKeepsRawPayload(t *testing.T) {
	t.Parallel()

	got := singleEnvelope(pkgsite.Result{
		Summary: map[string]any{
			"kind":       "package",
			"modulePath": "golang.org/x/oauth2",
		},
		Raw: map[string]any{
			"modulePath": "golang.org/x/oauth2",
			"version":    "v0.35.0",
		},
	}, envelopeOptions{})

	if !strings.Contains(got, `"raw"`) {
		t.Fatalf("single envelope should preserve raw payload:\n%s", got)
	}
	if !strings.Contains(got, `"version": "v0.35.0"`) {
		t.Fatalf("single envelope missing raw payload field:\n%s", got)
	}
}

func TestPaginatedEnvelopeReclaimsRawPayloadBudget(t *testing.T) {
	t.Parallel()

	result := pkgsite.Result{
		Summary: map[string]any{"kind": "symbols", "count": 5},
		Items: []map[string]any{
			{"name": "Config", "kind": "type"},
			{"name": "Token", "kind": "type"},
			{"name": "TokenSource", "kind": "func"},
			{"name": "NewClient", "kind": "func"},
			{"name": "ReuseTokenSource", "kind": "func"},
		},
		Raw: map[string]any{
			"symbols": strings.Repeat("raw symbol payload ", 260),
		},
	}
	page := pkgsite.PageInput{MaxTokens: 200}

	got := paginatedEnvelope(result, page, envelopeOptions{})
	old := paginatedEnvelopeWithRawForTest(result, page, envelopeOptions{})

	gotDisplayed := displayedItemsForTest(t, got)
	oldDisplayed := displayedItemsForTest(t, old)
	if gotDisplayed <= oldDisplayed {
		t.Fatalf("displayed_items = %d, want greater than old raw-budget baseline %d\ngot:\n%s\nold:\n%s", gotDisplayed, oldDisplayed, got, old)
	}
	if gotDisplayed != 5 {
		t.Fatalf("displayed_items = %d, want 5:\n%s", gotDisplayed, got)
	}
}

func TestPaginatedEnvelopeFormatsEmptyListWithoutRaw(t *testing.T) {
	t.Parallel()

	got := paginatedEnvelope(pkgsite.Result{
		Summary: map[string]any{
			"count":      0,
			"kind":       "vulnerabilities",
			"modulePath": "std",
			"path":       "slices",
		},
		Pagination: map[string]any{
			"upstreamNextPageToken": "",
		},
		Raw: map[string]any{"total": 0},
	}, pkgsite.PageInput{}, envelopeOptions{})

	tests := []struct {
		name string
		want string
	}{
		{name: "metadata count", want: "<count>0</count>"},
		{name: "displayed items", want: "<displayed_items>0</displayed_items>"},
		{name: "pagination displayed items", want: `"displayedItems": 0`},
		{name: "pagination total", want: `"total": 0`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(got, tt.want) {
				t.Fatalf("envelope missing %q:\n%s", tt.want, got)
			}
		})
	}
	if strings.Contains(got, `"raw"`) {
		t.Fatalf("empty paginated envelope should not contain raw payload:\n%s", got)
	}
}

func TestSliceEnvelopeUsesJSONDataAndNextCall(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"name": "Config"},
		{"name": "Token"},
	}
	for i := range 100 {
		items = append(items, map[string]any{"name": fmt.Sprintf("Symbol%d", i)})
	}
	got := sliceEnvelope(items, pkgsite.PageInput{MaxTokens: 200}, envelopeOptions{
		Source:   "pkg.go.dev/v1beta",
		ToolName: toolNameSymbols,
		NextArgs: map[string]any{"package_path": "golang.org/x/oauth2"},
	})

	tests := []struct {
		name string
		want string
	}{
		{name: "json data block", want: "<JSON_DATA>"},
		{name: "first item", want: `"name": "Config"`},
		{name: "truncated metadata", want: "<is_truncated>true</is_truncated>"},
		{name: "next call hint", want: "<next_call>pkgsite_symbols(package_path=golang.org/x/oauth2, start_at="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(got, tt.want) {
				t.Fatalf("envelope missing %q:\n%s", tt.want, got)
			}
		})
	}

	t.Run("no yaml data block", func(t *testing.T) {
		t.Parallel()
		if strings.Contains(got, "<YAML_DATA>") {
			t.Fatalf("envelope should not contain YAML data:\n%s", got)
		}
	})

	t.Run("no cache metadata", func(t *testing.T) {
		t.Parallel()
		for _, leaked := range []string{"from_cache", "FromCache", "cacheHit", "cache_hit"} {
			if strings.Contains(got, leaked) {
				t.Fatalf("envelope should not expose cache state %q:\n%s", leaked, got)
			}
		}
	})
}

func TestPaginatedEnvelopeNextCallHint(t *testing.T) {
	t.Parallel()

	got := paginatedEnvelope(pkgsite.Result{
		Items: []map[string]any{
			{"name": "Config"},
			{"name": "Token"},
			{"description": strings.Repeat("large symbol payload ", 200), "name": "NewClient"},
		},
	}, pkgsite.PageInput{StartAt: 1, MaxTokens: 400}, envelopeOptions{
		ToolName: toolNameSymbols,
		NextArgs: map[string]any{
			"module_path":  "",
			"package_path": "golang.org/x/oauth2",
			"version":      "v0.35.0",
		},
	})

	tests := []struct {
		name string
		want string
	}{
		{name: "truncated", want: "<is_truncated>true</is_truncated>"},
		{name: "next start", want: "<next_start_at>2</next_start_at>"},
		{name: "next call", want: "<next_call>pkgsite_symbols(package_path=golang.org/x/oauth2, start_at=2, version=v0.35.0)</next_call>"},
		{name: "pagination json hint", want: `"nextStartAt": 2`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(got, tt.want) {
				t.Fatalf("envelope missing %q:\n%s", tt.want, got)
			}
		})
	}
}

func paginatedEnvelopeWithRawForTest(result pkgsite.Result, page pkgsite.PageInput, opts envelopeOptions) string {
	if page.StartAt < 0 {
		page.StartAt = 0
	}
	if page.MaxTokens <= 0 {
		page.MaxTokens = defaultMaxTokens
	}
	if len(result.Items) == 0 {
		return singleEnvelope(result, opts)
	}

	total := len(result.Items)
	displayed := takeMaps(result.Items, page.StartAt, total-page.StartAt)
	if page.StartAt >= total {
		displayed = nil
	}
	maxChars := page.MaxTokens * 4
	next := page.StartAt + len(displayed)
	display := result
	display.Items = displayed
	display.Pagination = mergePagination(display.Pagination, total, len(displayed), page.StartAt, next)
	for len(displayed) > 0 && len(envelope(display, total, len(displayed), page.StartAt, next, opts, next < total)) > maxChars {
		displayed = displayed[:len(displayed)-1]
		next = page.StartAt + len(displayed)
		display.Items = displayed
		display.Pagination = mergePagination(display.Pagination, total, len(displayed), page.StartAt, next)
	}
	return envelope(display, total, len(displayed), page.StartAt, next, opts, next < total)
}

func displayedItemsForTest(t *testing.T, text string) int {
	t.Helper()

	const open = "<displayed_items>"
	const close = "</displayed_items>"
	start := strings.Index(text, open)
	if start < 0 {
		t.Fatalf("missing %s:\n%s", open, text)
	}
	start += len(open)
	end := strings.Index(text[start:], close)
	if end < 0 {
		t.Fatalf("missing %s:\n%s", close, text)
	}
	value, err := strconv.Atoi(text[start : start+end])
	if err != nil {
		t.Fatalf("parse displayed_items: %v", err)
	}
	return value
}

func diffStrings(want, got string) string {
	return fmt.Sprintf("want:\n%s\n\ngot:\n%s", want, got)
}
