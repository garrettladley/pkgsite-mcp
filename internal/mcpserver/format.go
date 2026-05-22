package mcpserver

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultMaxTokens = 10_000

type envelopeOptions struct {
	Source      string
	UpstreamURL string
	ToolName    string
	NextArgs    map[string]any
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func singleEnvelope(data any, opts envelopeOptions) string {
	return envelope(data, 1, 1, 0, 1, opts, false)
}

func paginatedEnvelope(result pkgsite.Result, page pkgsite.PageInput, opts envelopeOptions) string {
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

func sliceEnvelope(items any, page pkgsite.PageInput, opts envelopeOptions) string {
	if page.StartAt < 0 {
		page.StartAt = 0
	}
	if page.MaxTokens <= 0 {
		page.MaxTokens = defaultMaxTokens
	}
	total := sliceLen(items)
	displayed := takeSlice(items, page.StartAt, max(0, total-page.StartAt))
	displayedCount := sliceLen(displayed)
	next := page.StartAt + displayedCount
	maxChars := page.MaxTokens * 4
	for displayedCount > 0 && len(envelope(displayed, total, displayedCount, page.StartAt, next, opts, next < total)) > maxChars {
		displayedCount--
		displayed = takeSlice(items, page.StartAt, displayedCount)
		next = page.StartAt + displayedCount
	}
	return envelope(displayed, total, displayedCount, page.StartAt, next, opts, next < total)
}

func envelope(data any, total, displayed, startAt, next int, opts envelopeOptions, truncated bool) string {
	var b strings.Builder
	b.WriteString("<METADATA>\n")
	fmt.Fprintf(&b, "  <is_truncated>%t</is_truncated>\n", truncated)
	if truncated {
		fmt.Fprintf(&b, "  <truncation_message>Response truncated. Call again with start_at=%d to get the next batch, and/or increase max_tokens.</truncation_message>\n", next)
	}
	fmt.Fprintf(&b, "  <displayed_items>%d</displayed_items>\n", displayed)
	fmt.Fprintf(&b, "  <count>%d</count>\n", total)
	fmt.Fprintf(&b, "  <start_at>%d</start_at>\n", startAt)
	if truncated {
		fmt.Fprintf(&b, "  <next_start_at>%d</next_start_at>\n", next)
		if opts.ToolName != "" {
			fmt.Fprintf(&b, "  <next_call>%s</next_call>\n", formatNextCall(opts.ToolName, opts.NextArgs, next))
		}
	}
	if opts.Source != "" {
		fmt.Fprintf(&b, "  <source>%s</source>\n", opts.Source)
	}
	if opts.UpstreamURL != "" {
		fmt.Fprintf(&b, "  <upstream_url>%s</upstream_url>\n", opts.UpstreamURL)
	}
	b.WriteString("</METADATA>\n")
	b.WriteString("<JSON_DATA>\n")
	b.WriteString(toJSON(data))
	b.WriteString("\n</JSON_DATA>")
	return b.String()
}

func toJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func mergePagination(base map[string]any, total, displayed, startAt, next int) map[string]any {
	out := make(map[string]any, len(base)+4)
	maps.Copy(out, base)
	out["total"] = total
	out["displayedItems"] = displayed
	out["startAt"] = startAt
	if next < total {
		out["nextStartAt"] = next
	} else {
		out["nextStartAt"] = nil
	}
	return out
}

func formatNextCall(toolName string, args map[string]any, nextStartAt int) string {
	copied := make(map[string]any, len(args)+1)
	for key, value := range args {
		if value != "" && value != nil {
			copied[key] = value
		}
	}
	copied["start_at"] = nextStartAt
	keys := make([]string, 0, len(copied))
	for key := range copied {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, copied[key]))
	}
	return fmt.Sprintf("%s(%s)", toolName, strings.Join(parts, ", "))
}

func takeMaps(items []map[string]any, start, count int) []map[string]any {
	if start < 0 {
		start = 0
	}
	if start >= len(items) || count <= 0 {
		return nil
	}
	end := min(start+count, len(items))
	return items[start:end]
}

func sliceLen(items any) int {
	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		return 1
	}
	return v.Len()
}

func takeSlice(items any, start, count int) any {
	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		return items
	}
	if start > v.Len() {
		start = v.Len()
	}
	end := min(start+count, v.Len())
	return v.Slice(start, end).Interface()
}
