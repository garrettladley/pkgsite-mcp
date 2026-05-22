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
		page  *pkgsiteapi.PaginatedResponse
		count int
		want  map[string]any
	}{
		{
			name: "upstream total and next token",
			page: &pkgsiteapi.PaginatedResponse{
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
			page:  &pkgsiteapi.PaginatedResponse{},
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
		page *pkgsiteapi.PaginatedResponse
		want []map[string]any
	}{
		{name: "nil page", page: nil, want: nil},
		{name: "nil items", page: &pkgsiteapi.PaginatedResponse{}, want: nil},
		{
			name: "items",
			page: &pkgsiteapi.PaginatedResponse{Items: &[]map[string]any{
				{"name": "Config"},
				{"name": "Token"},
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
