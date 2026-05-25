package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/xcontext"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	MCPMethodInitialize    = "initialize"
	MCPMethodToolsCall     = "tools/call"
	MCPMethodPromptsGet    = "prompts/get"
	MCPMethodResourcesRead = "resources/read"

	MCPTransportHTTP    MCPTransport = "http"
	MCPTransportStdio   MCPTransport = "stdio"
	MCPTransportUnknown MCPTransport = "unknown"

	NetworkTransportPipe = "pipe"
	NetworkTransportTCP  = "tcp"

	jsonRPCVersion                = "2.0"
	maxToolResultContentBytes     = 4 << 10
	toolResultContentTruncation   = "..."
	minToolResultContentByteLimit = len(toolResultContentTruncation) + 1
)

type (
	MCPTransport        string
	MCPMessageDirection string
)

const (
	MCPDirectionClientToServer MCPMessageDirection = "client_to_server"
	MCPDirectionServerToClient MCPMessageDirection = "server_to_client"
)

type MCPServerInfo struct {
	Name    string
	Title   string
	Version string
}

type MCPServerTracingConfig struct {
	Transport MCPTransport
	Server    MCPServerInfo
}

type MCPToolResultAttrs struct {
	IsError      bool
	ContentCount int
	Content      string
}

func (a MCPToolResultAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Bool(AttrMCPToolResultIsError, a.IsError),
		attribute.Int(AttrMCPToolResultContentCount, a.ContentCount),
	}
	attrs = appendTrimmedString(attrs, AttrMCPToolResultContent, a.Content)
	return attrs
}

func MCPServerTracingMiddleware(cfg MCPServerTracingConfig) mcp.Middleware {
	return mcpServerTracingMiddleware(cfg, MCPDirectionClientToServer)
}

func MCPServerSendingTracingMiddleware(cfg MCPServerTracingConfig) mcp.Middleware {
	return mcpServerTracingMiddleware(cfg, MCPDirectionServerToClient)
}

func mcpServerTracingMiddleware(cfg MCPServerTracingConfig, direction MCPMessageDirection) mcp.Middleware {
	if cfg.Transport == "" {
		cfg.Transport = MCPTransportUnknown
	}
	if direction == "" {
		direction = MCPDirectionClientToServer
	}
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx, span := Tracer("mcp").Start(ctx, MCPServerSpanName(method, req),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(mcpServerSpanAttrs(ctx, method, req, cfg, direction)...),
			)
			defer span.End()

			result, err := next(ctx, method, req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				if method == MCPMethodToolsCall {
					span.SetAttributes(MCPToolResultAttrs{IsError: true}.Attributes()...)
				}
				return result, err
			}
			if method == MCPMethodToolsCall {
				span.SetAttributes(MCPToolResultAttrsFromResult(result).Attributes()...)
			}
			return result, nil
		}
	}
}

func MCPServerSpanName(method string, req mcp.Request) string {
	switch method {
	case MCPMethodToolsCall:
		if params, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && strings.TrimSpace(params.Name) != "" {
			return method + " " + strings.TrimSpace(params.Name)
		}
	case MCPMethodPromptsGet:
		if params, ok := req.GetParams().(*mcp.GetPromptParams); ok && strings.TrimSpace(params.Name) != "" {
			return method + " " + strings.TrimSpace(params.Name)
		}
	case MCPMethodResourcesRead:
		if params, ok := req.GetParams().(*mcp.ReadResourceParams); ok && strings.TrimSpace(params.URI) != "" {
			return method + " " + strings.TrimSpace(params.URI)
		}
	}
	return method
}

func MCPServerSpanAttrs(method string, req mcp.Request, cfg MCPServerTracingConfig) []attribute.KeyValue {
	return mcpServerSpanAttrs(context.Background(), method, req, cfg, MCPDirectionClientToServer)
}

func mcpServerSpanAttrs(ctx context.Context, method string, req mcp.Request, cfg MCPServerTracingConfig, direction MCPMessageDirection) []attribute.KeyValue {
	transport := cfg.Transport
	if transport == "" {
		transport = MCPTransportUnknown
	}
	if direction == "" {
		direction = MCPDirectionClientToServer
	}
	attrs := []attribute.KeyValue{
		attribute.String(AttrMCPMethodName, method),
		attribute.String(AttrMCPTransport, string(transport)),
		attribute.String(AttrNetworkTransport, transport.NetworkTransport()),
		attribute.String(AttrNetworkProtocolVersion, jsonRPCVersion),
	}
	if strings.HasPrefix(method, "notifications/") {
		attrs = append(attrs, attribute.String(AttrMCPNotificationDirection, string(direction)))
	}
	attrs = appendTrimmedString(attrs, AttrMCPServerName, cfg.Server.Name)
	attrs = appendTrimmedString(attrs, AttrMCPServerTitle, cfg.Server.Title)
	attrs = appendTrimmedString(attrs, AttrMCPServerVersion, cfg.Server.Version)
	if session := req.GetSession(); !isNilSession(session) {
		attrs = appendTrimmedString(attrs, AttrMCPSessionID, session.ID())
		if initParams := sessionInitializeParams(session); initParams != nil {
			attrs = appendInitializeTraceAttrs(attrs, initParams)
		}
	}
	if extra := req.GetExtra(); extra != nil {
		attrs = appendTrimmedString(attrs, AttrMCPProtocolVersionHeader, extra.Header.Get(xhttp.HeaderMCPProtocolVersion))
		attrs = appendTrimmedString(attrs, AttrMCPRequestID, extra.Header.Get(xhttp.HeaderInternalMCPRequestID))
		attrs = appendClientAddressAttrs(attrs, method, extra.Header.Get(xhttp.HeaderInternalMCPClientAddress), extra.Header.Get(xhttp.HeaderInternalMCPClientPort))
	} else if client, ok := xcontext.MCPClientFrom(ctx); ok {
		attrs = appendClientAddressAttrs(attrs, method, client.Address, strconv.Itoa(client.Port))
	}

	switch method {
	case MCPMethodInitialize:
		if params, ok := req.GetParams().(*mcp.InitializeParams); ok && params != nil {
			attrs = appendInitializeTraceAttrs(attrs, params)
		}
	case MCPMethodToolsCall:
		if params, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && params != nil {
			attrs = appendTrimmedString(attrs, AttrMCPToolName, params.Name)
			attrs = append(attrs, MCPRequestArgumentAttrs(params.Arguments)...)
		}
	case MCPMethodPromptsGet:
		if params, ok := req.GetParams().(*mcp.GetPromptParams); ok && params != nil {
			attrs = appendTrimmedString(attrs, AttrMCPPromptName, params.Name)
			attrs = append(attrs, MCPRequestArgumentAttrs(params.Arguments)...)
		}
	case MCPMethodResourcesRead:
		if params, ok := req.GetParams().(*mcp.ReadResourceParams); ok && params != nil {
			attrs = appendTrimmedString(attrs, AttrMCPResourceURI, params.URI)
		}
	}
	return attrs
}

func MCPRequestArgumentAttrs(args any) []attribute.KeyValue {
	argsMap := map[string]any{}
	switch typed := args.(type) {
	case nil:
		return nil
	case json.RawMessage:
		if len(typed) == 0 {
			return nil
		}
		if err := json.Unmarshal(typed, &argsMap); err != nil {
			return nil
		}
	case []byte:
		if len(typed) == 0 {
			return nil
		}
		if err := json.Unmarshal(typed, &argsMap); err != nil {
			return nil
		}
	case map[string]string:
		attrs := make([]attribute.KeyValue, 0, len(typed))
		for key, value := range typed {
			attrs = append(attrs, attribute.String(AttrMCPRequestArgumentPrefix+key, value))
		}
		return attrs
	case map[string]any:
		argsMap = typed
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(encoded, &argsMap); err != nil {
			return nil
		}
	}

	attrs := make([]attribute.KeyValue, 0, len(argsMap))
	for key, value := range argsMap {
		if key = strings.TrimSpace(key); key != "" {
			attrs = append(attrs, attribute.String(AttrMCPRequestArgumentPrefix+key, mcpArgumentValue(value)))
		}
	}
	return attrs
}

func MCPToolResultAttrsFromResult(result mcp.Result) MCPToolResultAttrs {
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok || toolResult == nil {
		return MCPToolResultAttrs{}
	}
	var content string
	if encoded, err := json.Marshal(toolResult.Content); err == nil {
		content = truncateToolResultContent(string(encoded), maxToolResultContentBytes)
	}
	return MCPToolResultAttrs{
		IsError:      toolResult.IsError,
		ContentCount: len(toolResult.Content),
		Content:      content,
	}
}

func truncateToolResultContent(content string, maxBytes int) string {
	if len(content) <= maxBytes {
		return content
	}
	if maxBytes < minToolResultContentByteLimit {
		return ""
	}

	limit := maxBytes - len(toolResultContentTruncation)
	cut := 0
	for idx := range content {
		if idx > limit {
			break
		}
		cut = idx
	}
	if cut == 0 {
		cut = limit
	}
	return content[:cut] + toolResultContentTruncation
}

func (t MCPTransport) NetworkTransport() string {
	switch t {
	case MCPTransportHTTP:
		return NetworkTransportTCP
	case MCPTransportStdio:
		return NetworkTransportPipe
	default:
		return "unknown"
	}
}

type initializeParamsProvider interface {
	InitializeParams() *mcp.InitializeParams
}

func sessionInitializeParams(session mcp.Session) *mcp.InitializeParams {
	if isNilSession(session) {
		return nil
	}
	provider, ok := session.(initializeParamsProvider)
	if !ok {
		return nil
	}
	return provider.InitializeParams()
}

func isNilSession(session mcp.Session) bool {
	if session == nil {
		return true
	}
	value := reflect.ValueOf(session)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func appendInitializeTraceAttrs(attrs []attribute.KeyValue, params *mcp.InitializeParams) []attribute.KeyValue {
	if params == nil {
		return attrs
	}
	attrs = appendTrimmedString(attrs, AttrMCPProtocolVersion, params.ProtocolVersion)
	if params.ClientInfo != nil {
		attrs = appendTrimmedString(attrs, AttrMCPClientName, params.ClientInfo.Name)
		attrs = appendTrimmedString(attrs, AttrMCPClientTitle, params.ClientInfo.Title)
		attrs = appendTrimmedString(attrs, AttrMCPClientVersion, params.ClientInfo.Version)
	}
	return attrs
}

func appendClientAddressAttrs(attrs []attribute.KeyValue, method, address, portValue string) []attribute.KeyValue {
	if method != MCPMethodInitialize {
		return attrs
	}
	attrs = appendTrimmedString(attrs, AttrClientAddress, address)
	if port, err := strconv.Atoi(strings.TrimSpace(portValue)); err == nil && port > 0 {
		attrs = append(attrs, attribute.Int(AttrClientPort, port))
	}
	return attrs
}

func mcpArgumentValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}
