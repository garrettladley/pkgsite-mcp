package observability

import (
	"context"
	"sync/atomic"

	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/metric"
)

var mcpMetrics = newMCPMetrics()

type mcpMetricSet struct {
	sink atomic.Pointer[metricSinkHolder]

	initializes metric.Int64Counter
	discovers   metric.Int64Counter
}

func newMCPMetrics() *mcpMetricSet {
	meter := Meter("mcp")
	initializes, _ := meter.Int64Counter("mcp.initialize.count")
	discovers, _ := meter.Int64Counter("mcp.discover.count")
	return &mcpMetricSet{initializes: initializes, discovers: discovers}
}

func initMCPMetrics(sink MetricSink) {
	mcpMetrics.init(sink)
}

func RecordMCPInitialize(ctx context.Context, attrs InitializeAttrs) {
	mcpMetrics.recordInitialize(ctx, attrs)
}

func RecordMCPDiscover(ctx context.Context, attrs InitializeAttrs) {
	mcpMetrics.recordDiscover(ctx, attrs)
}

// MCPRequestClientAttrs reads modern per-request metadata and falls back to the
// SDK's negotiated session metadata for legacy requests.
func MCPRequestClientAttrs(req mcp.Request) InitializeAttrs {
	var attrs InitializeAttrs
	var info *mcp.Implementation
	params := req.GetParams()
	if initialized, ok := params.(*mcp.InitializeParams); ok && initialized != nil {
		attrs.ProtocolVersion, info = initialized.ProtocolVersion, initialized.ClientInfo
	} else if provider, ok := req.(interface {
		ProtocolVersion() string
		ClientInfo() *mcp.Implementation
	}); ok {
		attrs.ProtocolVersion, info = provider.ProtocolVersion(), provider.ClientInfo()
		// clientInfo is optional on modern requests; never inherit another
		// request's identity if it was omitted on a persistent transport.
		if attrs.ProtocolVersion >= "2026-07-28" {
			if _, present := params.GetMeta()[mcp.MetaKeyClientInfo]; !present {
				info = nil
			}
		}
	}
	if info != nil {
		attrs.ClientName, attrs.ClientTitle, attrs.ClientVersion = info.Name, info.Title, info.Version
	}
	if extra := req.GetExtra(); extra != nil {
		attrs.ProtocolVersionHeader = extra.Header.Get(xhttp.HeaderMCPProtocolVersion)
	}
	return attrs
}

func (m *mcpMetricSet) init(sink MetricSink) {
	if sink == nil {
		m.sink.Store(nil)
		return
	}
	m.sink.Store(&metricSinkHolder{sink: sink})
}

func (m *mcpMetricSet) recordInitialize(ctx context.Context, attrs InitializeAttrs) {
	metricAttrs := attrs.Attributes()
	m.initializes.Add(ctx, 1, metric.WithAttributes(metricAttrs...))

	if holder := m.sink.Load(); holder != nil {
		holder.sink.Count(ctx, "mcp.initialize", 1, metricAttrs...)
	}
}

func (m *mcpMetricSet) recordDiscover(ctx context.Context, attrs InitializeAttrs) {
	metricAttrs := attrs.Attributes()
	m.discovers.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
	if holder := m.sink.Load(); holder != nil {
		holder.sink.Count(ctx, "mcp.discover", 1, metricAttrs...)
	}
}
