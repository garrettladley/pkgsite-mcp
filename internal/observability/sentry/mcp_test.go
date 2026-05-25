package sentry

import (
	"slices"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMCPReadOnlySpanAddsSentryOpMapping(t *testing.T) {
	t.Parallel()

	span := mcpReadOnlySpan{
		ReadOnlySpan: tracetest.SpanStub{
			Name: "tools/call pkgsite_search",
			Attributes: []attribute.KeyValue{
				attribute.String(observability.AttrMCPMethodName, observability.MCPMethodToolsCall),
			},
		}.Snapshot(),
	}

	attrs := span.Attributes()
	assertStringAttr(t, attrs, "faas.trigger", mcpSpanOpServer)
	assertStringAttr(t, attrs, observability.AttrMCPMethodName, observability.MCPMethodToolsCall)
}

func TestMCPReadOnlySpanAddsClientToServerNotificationOpMapping(t *testing.T) {
	t.Parallel()

	span := mcpReadOnlySpan{
		ReadOnlySpan: tracetest.SpanStub{
			Name: "notifications/cancelled",
			Attributes: []attribute.KeyValue{
				attribute.String(observability.AttrMCPMethodName, "notifications/cancelled"),
			},
		}.Snapshot(),
	}

	assertStringAttr(t, span.Attributes(), "faas.trigger", mcpSpanOpNotificationClientToServer)
}

func TestMCPReadOnlySpanAddsServerToClientNotificationOpMapping(t *testing.T) {
	t.Parallel()

	span := mcpReadOnlySpan{
		ReadOnlySpan: tracetest.SpanStub{
			Name: "notifications/progress",
			Attributes: []attribute.KeyValue{
				attribute.String(observability.AttrMCPMethodName, "notifications/progress"),
				attribute.String(observability.AttrMCPNotificationDirection, string(observability.MCPDirectionServerToClient)),
			},
		}.Snapshot(),
	}

	assertStringAttr(t, span.Attributes(), "faas.trigger", mcpSpanOpNotificationServerToClient)
}

func TestMCPReadOnlySpanSkipsNonMCPSpans(t *testing.T) {
	t.Parallel()

	span := mcpReadOnlySpan{
		ReadOnlySpan: tracetest.SpanStub{
			Name:       "pkgsite.cache lookup",
			Attributes: []attribute.KeyValue{attribute.String("pkgsite.cache.outcome", "hit")},
		}.Snapshot(),
	}

	attrs := span.Attributes()
	if slices.ContainsFunc(attrs, func(attr attribute.KeyValue) bool {
		return string(attr.Key) == "faas.trigger"
	}) {
		t.Fatalf("faas.trigger present on non-MCP span")
	}
}

func assertStringAttr(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()

	for _, attr := range attrs {
		if string(attr.Key) == key {
			if got := attr.Value.AsString(); got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s missing from attrs", key)
}
