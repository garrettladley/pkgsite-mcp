package mcpserver

import (
	"fmt"
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
  },
  "raw": {
    "nextPageToken": "upstream-token"
  }
}
</JSON_DATA>`

	if got != want {
		t.Fatalf("envelope mismatch (-want +got):\n%s", diffStrings(want, got))
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

func TestLoadSkillParsesHeader(t *testing.T) {
	t.Parallel()

	skill := loadSkillForTest(t, "pkgsite/overview")
	tests := []struct {
		name  string
		check func(*testing.T)
	}{
		{name: "name", check: func(t *testing.T) {
			t.Helper()
			if skill.Name != "pkgsite/overview" {
				t.Fatalf("Name = %q", skill.Name)
			}
		}},
		{name: "description", check: func(t *testing.T) {
			t.Helper()
			if skill.Description == "" {
				t.Fatal("Description is empty")
			}
		}},
		{name: "related", check: func(t *testing.T) {
			t.Helper()
			if len(skill.Related) == 0 {
				t.Fatal("Related is empty")
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t)
		})
	}
}

func loadSkillForTest(t *testing.T, name string) Skill {
	t.Helper()

	skill, err := LoadSkill(name)
	if err != nil {
		t.Fatal(err)
	}
	return skill
}

func diffStrings(want, got string) string {
	return fmt.Sprintf("want:\n%s\n\ngot:\n%s", want, got)
}
