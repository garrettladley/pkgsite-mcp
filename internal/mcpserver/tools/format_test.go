package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
)

func TestPaginateKeepsUpstreamAndLocalCountsSeparate(t *testing.T) {
	t.Parallel()
	result := pkgsite.Result{
		Summary:    map[string]any{"packagePath": "net/http", "modulePath": "std", "version": "v1.27.2"},
		Items:      []map[string]any{{"name": "Client"}, {"name": "Client.Do"}, {"name": "Client.Get"}},
		Pagination: map[string]any{"total": 406, "upstreamNextPageToken": "upstream-token"},
	}
	output := symbolsOutputFromResult(result)
	if err := paginate(t.Context(), result, pkgsite.PageInput{StartAt: 1}, &output, &output.listOutput); err != nil {
		t.Fatal(err)
	}
	page := output.Pagination
	if page.FetchedItems != 3 || page.DisplayedItems != 2 || page.StartAt != 1 || page.NextStartAt != nil || page.Total == nil || *page.Total != 406 || page.UpstreamNextPageToken != "upstream-token" {
		t.Fatalf("pagination = %+v", page)
	}
	if len(output.Items) != 2 || *output.Items[0].Name != "Client.Do" {
		t.Fatalf("items = %+v", output.Items)
	}
}

func TestPaginateBudgetAndContinuation(t *testing.T) {
	t.Parallel()
	result := pkgsite.Result{Items: []map[string]any{
		{"name": "Client", "synopsis": strings.Repeat("a", 300)},
		{"name": "Client.Do", "synopsis": strings.Repeat("b", 300)},
		{"name": "Client.Get", "synopsis": strings.Repeat("c", 300)},
	}, Pagination: map[string]any{"upstreamNextPageToken": "upstream-token"}}
	output := symbolsOutputFromResult(result)
	page := pkgsite.PageInput{MaxTokens: 210}
	if err := paginate(t.Context(), result, page, &output, &output.listOutput); err != nil {
		t.Fatal(err)
	}
	if len(output.Items) != 1 || output.Pagination.NextStartAt == nil || *output.Pagination.NextStartAt != 1 {
		t.Fatalf("first page = %+v", output)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > page.MaxTokens*4 {
		t.Fatalf("response exceeds budget: %d bytes", len(data))
	}
	page.StartAt = *output.Pagination.NextStartAt
	if err := paginate(t.Context(), result, page, &output, &output.listOutput); err != nil {
		t.Fatal(err)
	}
	if *output.Items[0].Name != "Client.Do" {
		t.Fatalf("next page starts with %q", *output.Items[0].Name)
	}
	if output.Pagination.UpstreamNextPageToken != "upstream-token" {
		t.Fatal("lost upstream token")
	}
}

func TestPaginateEmptyAndOutOfRange(t *testing.T) {
	t.Parallel()
	for _, start := range []int{-1, 0, 10} {
		output := symbolsOutput{}
		if err := paginate(t.Context(), pkgsite.Result{}, pkgsite.PageInput{StartAt: start}, &output, &output.listOutput); err != nil {
			t.Fatal(err)
		}
		if output.Items == nil || len(output.Items) != 0 || output.Pagination.NextStartAt != nil {
			t.Fatalf("empty page = %+v", output)
		}
	}
}

func TestPaginateRejectsBudgetThatCannotMakeProgress(t *testing.T) {
	t.Parallel()
	result := pkgsite.Result{Items: []map[string]any{{"name": "Client", "synopsis": strings.Repeat("large", 100)}}}
	output := symbolsOutputFromResult(result)
	err := paginate(t.Context(), result, pkgsite.PageInput{MaxTokens: 20}, &output, &output.listOutput)
	if err == nil || !strings.Contains(err.Error(), "increase max_tokens") {
		t.Fatalf("error = %v", err)
	}
}
