package sentry

import (
	"context"
	"slices"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	mcpSpanOpServer                     = "mcp.server"
	mcpSpanOpNotificationClientToServer = "mcp.notification.client_to_server"
	mcpSpanOpNotificationServerToClient = "mcp.notification.server_to_client"
)

type mcpSpanProcessor struct {
	delegate sdktrace.SpanProcessor
}

var _ sdktrace.SpanProcessor = (*mcpSpanProcessor)(nil)

func newMCPSpanProcessor() sdktrace.SpanProcessor {
	//nolint:staticcheck // sentry-go's native processor is currently the only supported path that maps faas.trigger to Sentry span op.
	return &mcpSpanProcessor{delegate: sentryotel.NewSentrySpanProcessor()}
}

func (p *mcpSpanProcessor) OnStart(parent context.Context, span sdktrace.ReadWriteSpan) {
	p.delegate.OnStart(parent, span)
}

func (p *mcpSpanProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	p.delegate.OnEnd(mcpReadOnlySpan{ReadOnlySpan: span})
}

func (p *mcpSpanProcessor) Shutdown(ctx context.Context) error {
	return p.delegate.Shutdown(ctx)
}

func (p *mcpSpanProcessor) ForceFlush(ctx context.Context) error {
	return p.delegate.ForceFlush(ctx)
}

type mcpReadOnlySpan struct {
	sdktrace.ReadOnlySpan
}

func (s mcpReadOnlySpan) Attributes() []attribute.KeyValue {
	attrs := s.ReadOnlySpan.Attributes()
	method := mcpMethodName(attrs)
	if method == "" {
		return attrs
	}
	// sentry-go/otel maps faas.trigger onto the native Sentry span op. Keep
	// that bridge local to the Sentry backend so generic MCP spans stay clean.
	return append(slices.Clone(attrs), attribute.String("faas.trigger", mcpSpanOp(method, mcpNotificationDirection(attrs))))
}

func mcpMethodName(attrs []attribute.KeyValue) string {
	for _, attr := range attrs {
		if string(attr.Key) == observability.AttrMCPMethodName {
			return attr.Value.AsString()
		}
	}
	return ""
}

func mcpNotificationDirection(attrs []attribute.KeyValue) string {
	for _, attr := range attrs {
		if string(attr.Key) == observability.AttrMCPNotificationDirection {
			return attr.Value.AsString()
		}
	}
	return ""
}

func mcpSpanOp(method, direction string) string {
	if strings.HasPrefix(method, "notifications/") {
		if direction == string(observability.MCPDirectionServerToClient) {
			return mcpSpanOpNotificationServerToClient
		}
		return mcpSpanOpNotificationClientToServer
	}
	return mcpSpanOpServer
}
