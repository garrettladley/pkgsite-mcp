package pkgsite

import (
	"reflect"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsiteapi"
)

func TestPaginationMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		page  any
		count int
		want  map[string]any
	}{
		{
			name: "upstream total and next token",
			page: &pkgsiteapi.PaginatedResponseSymbol{
				NextPageToken: new("next-page-token"),
				Total:         new(42),
			},
			count: 3,
			want: map[string]any{
				"total":                 42,
				"displayedItems":        3,
				"startAt":               0,
				"nextStartAt":           nil,
				"upstreamNextPageToken": "next-page-token",
			},
		},
		{
			name:  "nil page uses displayed count as total",
			page:  nil,
			count: 2,
			want: map[string]any{
				"total":                 2,
				"displayedItems":        2,
				"startAt":               0,
				"nextStartAt":           nil,
				"upstreamNextPageToken": "",
			},
		},
		{
			name:  "missing upstream token is empty string",
			page:  &pkgsiteapi.PaginatedResponseSymbol{},
			count: 0,
			want: map[string]any{
				"total":                 0,
				"displayedItems":        0,
				"startAt":               0,
				"nextStartAt":           nil,
				"upstreamNextPageToken": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := pagination(tt.page, tt.count)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pagination() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPaginatedItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page any
		want []map[string]any
	}{
		{name: "nil page", page: nil, want: nil},
		{name: "nil items", page: &pkgsiteapi.PaginatedResponseSymbol{}, want: nil},
		{
			name: "items",
			page: &pkgsiteapi.PaginatedResponseSymbol{Items: &[]pkgsiteapi.Symbol{
				{Name: new("Config")},
				{Name: new("Token")},
			}},
			want: []map[string]any{
				{"name": "Config"},
				{"name": "Token"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := paginatedItems(tt.page)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("paginatedItems() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPaginatedItemsUsesGeneratedModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page any
		want []map[string]any
	}{
		{
			name: "module versions",
			page: &pkgsiteapi.PaginatedResponseModuleVersion{Items: &[]pkgsiteapi.ModuleVersion{{
				ModulePath: new("example.com/module"), LatestVersion: new("v1.2.3"),
			}}},
			want: []map[string]any{{"modulePath": "example.com/module", "latestVersion": "v1.2.3"}},
		},
		{
			name: "package info",
			page: &pkgsiteapi.PaginatedResponsePackageInfo{Items: &[]pkgsiteapi.PackageInfo{{
				Path: new("example.com/module/pkg"), Name: new("pkg"),
			}}},
			want: []map[string]any{{"name": "pkg", "path": "example.com/module/pkg"}},
		},
		{
			name: "search results",
			page: &pkgsiteapi.PaginatedResponseSearchResult{Items: &[]pkgsiteapi.SearchResult{{
				PackagePath: new("example.com/module/pkg"), ModulePath: new("example.com/module"),
			}}},
			want: []map[string]any{{"modulePath": "example.com/module", "packagePath": "example.com/module/pkg"}},
		},
		{
			name: "symbols",
			page: &pkgsiteapi.PaginatedResponseSymbol{Items: &[]pkgsiteapi.Symbol{{
				Name: new("Config"), Kind: new("Type"),
			}}},
			want: []map[string]any{{"kind": "Type", "name": "Config"}},
		},
		{
			name: "vulnerabilities",
			page: &pkgsiteapi.PaginatedResponseVulnerability{Items: &[]pkgsiteapi.Vulnerability{{
				Id: new("GO-2026-0001"), Summary: new("example vulnerability"),
			}}},
			want: []map[string]any{{"id": "GO-2026-0001", "summary": "example vulnerability"}},
		},
		{
			name: "strings",
			page: &pkgsiteapi.PaginatedResponseString{Items: &[]string{"example.com/importer"}},
			want: []map[string]any{{"path": "example.com/importer"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := paginatedItems(tt.page); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("paginatedItems() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
