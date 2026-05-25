package observability

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/garrettladley/pkgsite-mcp/internal/xcontext"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
)

func TestMCPServerSpanAttrsToolCall(t *testing.T) {
	t.Parallel()

	req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
		Params: &mcp.CallToolParamsRaw{
			Name:      "pkgsite_search",
			Arguments: json.RawMessage(`{"query":"slices","limit":5,"include_imports":true}`),
		},
		Extra: &mcp.RequestExtra{Header: http.Header{xhttp.HeaderInternalMCPRequestID: {"1"}}},
	}
	attrs := MCPServerSpanAttrs(MCPMethodToolsCall, req, MCPServerTracingConfig{
		Transport: MCPTransportHTTP,
		Server:    MCPServerInfo{Name: "pkgsite", Title: "pkg.go.dev MCP", Version: "sha-abc123"},
	})

	if got, want := MCPServerSpanName(MCPMethodToolsCall, req), "tools/call pkgsite_search"; got != want {
		t.Fatalf("MCPServerSpanName() = %q, want %q", got, want)
	}
	assertStringAttr(t, attrs, AttrMCPMethodName, MCPMethodToolsCall)
	assertStringAttr(t, attrs, AttrMCPTransport, string(MCPTransportHTTP))
	assertStringAttr(t, attrs, AttrMCPRequestID, "1")
	assertStringAttr(t, attrs, AttrNetworkTransport, NetworkTransportTCP)
	assertStringAttr(t, attrs, AttrNetworkProtocolVersion, jsonRPCVersion)
	assertStringAttr(t, attrs, AttrMCPServerName, "pkgsite")
	assertStringAttr(t, attrs, AttrMCPServerTitle, "pkg.go.dev MCP")
	assertStringAttr(t, attrs, AttrMCPServerVersion, "sha-abc123")
	assertStringAttr(t, attrs, AttrMCPToolName, "pkgsite_search")
	assertStringAttr(t, attrs, AttrMCPRequestArgumentPrefix+"query", "slices")
	assertStringAttr(t, attrs, AttrMCPRequestArgumentPrefix+"limit", "5")
	assertStringAttr(t, attrs, AttrMCPRequestArgumentPrefix+"include_imports", "true")
}

func TestMCPServerSpanAttrsInitialize(t *testing.T) {
	t.Parallel()

	req := &mcp.ClientRequest[*mcp.InitializeParams]{
		Params: &mcp.InitializeParams{
			ProtocolVersion: "2025-06-18",
			ClientInfo:      &mcp.Implementation{Name: "codex-mcp-client", Title: "Codex", Version: "1.2.3"},
		},
	}
	ctx := xcontext.WithMCPClient(t.Context(), xcontext.MCPClient{Address: "203.0.113.9", Port: 54321})
	attrs := mcpServerSpanAttrs(ctx, MCPMethodInitialize, req, MCPServerTracingConfig{Transport: MCPTransportStdio}, MCPDirectionClientToServer)

	if got, want := MCPServerSpanName(MCPMethodInitialize, req), "initialize"; got != want {
		t.Fatalf("MCPServerSpanName() = %q, want %q", got, want)
	}
	assertStringAttr(t, attrs, AttrMCPTransport, string(MCPTransportStdio))
	assertStringAttr(t, attrs, AttrNetworkTransport, NetworkTransportPipe)
	assertStringAttr(t, attrs, AttrMCPProtocolVersion, "2025-06-18")
	assertStringAttr(t, attrs, AttrMCPClientName, "codex-mcp-client")
	assertStringAttr(t, attrs, AttrMCPClientTitle, "Codex")
	assertStringAttr(t, attrs, AttrMCPClientVersion, "1.2.3")
	assertStringAttr(t, attrs, AttrClientAddress, "203.0.113.9")
	assertIntAttr(t, attrs, AttrClientPort, 54321)
}

func TestMCPServerSpanAttrsInitializeDoesNotDuplicateClientAddress(t *testing.T) {
	t.Parallel()

	req := &mcp.ServerRequest[*mcp.InitializeParams]{
		Params: &mcp.InitializeParams{ProtocolVersion: "2025-06-18"},
		Extra: &mcp.RequestExtra{Header: http.Header{
			xhttp.HeaderInternalMCPClientAddress: {"203.0.113.9"},
			xhttp.HeaderInternalMCPClientPort:    {"54321"},
		}},
	}
	ctx := xcontext.WithMCPClient(t.Context(), xcontext.MCPClient{Address: "203.0.113.9", Port: 54321})
	attrs := mcpServerSpanAttrs(ctx, MCPMethodInitialize, req, MCPServerTracingConfig{}, MCPDirectionClientToServer)

	assertAttrCount(t, attrs, AttrClientAddress, 1)
	assertAttrCount(t, attrs, AttrClientPort, 1)
}

func TestMCPServerSpanNamePromptAndResource(t *testing.T) {
	t.Parallel()

	promptReq := &mcp.ServerRequest[*mcp.GetPromptParams]{Params: &mcp.GetPromptParams{Name: "analyze-code"}}
	if got, want := MCPServerSpanName(MCPMethodPromptsGet, promptReq), "prompts/get analyze-code"; got != want {
		t.Fatalf("prompt span name = %q, want %q", got, want)
	}

	resourceReq := &mcp.ServerRequest[*mcp.ReadResourceParams]{Params: &mcp.ReadResourceParams{URI: "file:///tmp/readme.md"}}
	if got, want := MCPServerSpanName(MCPMethodResourcesRead, resourceReq), "resources/read file:///tmp/readme.md"; got != want {
		t.Fatalf("resource span name = %q, want %q", got, want)
	}
	attrs := MCPServerSpanAttrs(MCPMethodResourcesRead, resourceReq, MCPServerTracingConfig{})
	assertStringAttr(t, attrs, AttrMCPResourceURI, "file:///tmp/readme.md")
}

func TestMCPServerSpanAttrsNotificationDirection(t *testing.T) {
	t.Parallel()

	receivingAttrs := MCPServerSpanAttrs("notifications/cancelled", &mcp.ClientRequest[*mcp.CancelledParams]{}, MCPServerTracingConfig{})
	assertStringAttr(t, receivingAttrs, AttrMCPNotificationDirection, string(MCPDirectionClientToServer))

	sendingAttrs := mcpServerSpanAttrs(t.Context(), "notifications/progress", &mcp.ServerRequest[*mcp.ProgressNotificationParams]{}, MCPServerTracingConfig{}, MCPDirectionServerToClient)
	assertStringAttr(t, sendingAttrs, AttrMCPNotificationDirection, string(MCPDirectionServerToClient))
}

func TestMCPToolResultAttrsFromResult(t *testing.T) {
	t.Parallel()

	attrs := MCPToolResultAttrsFromResult(&mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: "one"},
			&mcp.TextContent{Text: "two"},
		},
	}).Attributes()

	assertBoolAttr(t, attrs, AttrMCPToolResultIsError, true)
	assertIntAttr(t, attrs, AttrMCPToolResultContentCount, 2)
	assertStringAttr(t, attrs, AttrMCPToolResultContent, `[{"type":"text","text":"one"},{"type":"text","text":"two"}]`)
}

func TestTruncateToolResultContent(t *testing.T) {
	t.Parallel()

	got := truncateToolResultContent("0123456789", 8)
	if want := "01234..."; got != want {
		t.Fatalf("truncateToolResultContent() = %q, want %q", got, want)
	}
	if got := truncateToolResultContent("short", 8); got != "short" {
		t.Fatalf("truncateToolResultContent() = %q, want short", got)
	}
}

func assertAttrCount(t *testing.T, attrs []attribute.KeyValue, key string, want int) {
	t.Helper()

	var got int
	for _, attr := range attrs {
		if string(attr.Key) == key {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", key, got, want)
	}
}

func assertBoolAttr(t *testing.T, attrs []attribute.KeyValue, key string, want bool) {
	t.Helper()

	for _, attr := range attrs {
		if string(attr.Key) == key {
			if got := attr.Value.AsBool(); got != want {
				t.Fatalf("%s = %t, want %t", key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s missing from attrs", key)
}

func assertIntAttr(t *testing.T, attrs []attribute.KeyValue, key string, want int64) {
	t.Helper()

	for _, attr := range attrs {
		if string(attr.Key) == key {
			if got := attr.Value.AsInt64(); got != want {
				t.Fatalf("%s = %d, want %d", key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s missing from attrs", key)
}
