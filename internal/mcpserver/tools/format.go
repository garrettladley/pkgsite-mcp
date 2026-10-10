package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

const defaultMaxTokens = 10_000

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func decodeOutput[T any](value any) (T, error) {
	var output T
	data, err := json.Marshal(value)
	if err != nil {
		return output, fmt.Errorf("encode pkgsite result: %w", err)
	}
	if err := json.Unmarshal(data, &output); err != nil {
		return output, fmt.Errorf("decode pkgsite result: %w", err)
	}
	return output, nil
}

// paginate budgets the complete JSON payload, including lookup context and source.
// The SDK serializes this same output into structuredContent and its text fallback.
func paginate[T any](ctx context.Context, result pkgsite.Result, page pkgsite.PageInput, output any, list *listOutput[T]) error {
	if result.Error != nil {
		list.Items = []T{}
		return nil
	}
	items, err := decodeOutput[[]T](result.Items)
	if err != nil {
		return err
	}
	start := max(0, page.StartAt)
	list.Items = items[min(start, len(items)):]
	if list.Items == nil {
		list.Items = []T{}
	}
	list.Pagination = &paginationOutput{
		FetchedItems: len(items), StartAt: start,
		UpstreamNextPageToken: summaryString(result.Pagination, "upstreamNextPageToken"),
	}
	if total, ok := result.Pagination["total"].(int); ok {
		list.Pagination.Total = &total
	}
	maxTokens := page.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	for {
		list.Pagination.DisplayedItems = len(list.Items)
		next := start + len(list.Items)
		list.Pagination.NextStartAt = nil
		if next < len(items) {
			list.Pagination.NextStartAt = &next
		}
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("encode tool output: %w", err)
		}
		if (len(data)+3)/4 <= maxTokens {
			break
		}
		if len(list.Items) <= 1 {
			return fmt.Errorf("max_tokens=%d is too small for this response; increase max_tokens", maxTokens)
		}
		list.Items = list.Items[:len(list.Items)-1]
	}
	nextStart := 0
	if list.Pagination.NextStartAt != nil {
		nextStart = *list.Pagination.NextStartAt
	}
	trace.SpanFromContext(ctx).SetAttributes(observability.EnvelopeAttrs{
		DisplayedItems: len(list.Items), TotalItems: len(items), StartAt: start,
		NextStartAt: nextStart, MaxTokens: maxTokens,
		Truncated: list.Pagination.NextStartAt != nil,
	}.Attributes()...)
	return nil
}
