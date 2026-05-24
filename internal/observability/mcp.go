package observability

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

var mcpMetrics = newMCPMetrics()

type mcpMetricSet struct {
	sink atomic.Pointer[metricSinkHolder]

	initializes metric.Int64Counter
}

func newMCPMetrics() *mcpMetricSet {
	meter := Meter("mcp")
	initializes, _ := meter.Int64Counter("mcp.initialize.count")
	return &mcpMetricSet{initializes: initializes}
}

func initMCPMetrics(sink MetricSink) {
	mcpMetrics.init(sink)
}

func RecordMCPInitialize(ctx context.Context, attrs InitializeAttrs) {
	mcpMetrics.recordInitialize(ctx, attrs)
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
