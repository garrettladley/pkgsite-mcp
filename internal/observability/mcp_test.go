package observability

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
)

func TestRecordMCPInitializeWritesMetricSink(t *testing.T) {
	t.Parallel()

	sink := &recordingMetricSink{}
	metrics := newMCPMetrics()
	metrics.init(sink)

	metrics.recordInitialize(t.Context(), InitializeAttrs{
		ClientName:      "codex-mcp-client",
		ClientTitle:     "Codex",
		ClientVersion:   "1.2.3",
		ProtocolVersion: "2025-06-18",
	})

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if got, want := len(sink.counts), 1; got != want {
		t.Fatalf("metric count = %d, want %d", got, want)
	}
	record := sink.counts[0]
	if got, want := record.name, "mcp.initialize"; got != want {
		t.Fatalf("metric name = %q, want %q", got, want)
	}
	if got, want := record.value, int64(1); got != want {
		t.Fatalf("metric value = %d, want %d", got, want)
	}
	assertStringAttr(t, record.attrs, AttrMCPClientName, "codex-mcp-client")
	assertStringAttr(t, record.attrs, AttrMCPClientTitle, "Codex")
	assertStringAttr(t, record.attrs, AttrMCPClientVersion, "1.2.3")
	assertStringAttr(t, record.attrs, AttrMCPProtocolVersion, "2025-06-18")
}

func TestRecordMCPDiscoverWritesSeparateMetric(t *testing.T) {
	t.Parallel()
	sink := &recordingMetricSink{}
	metrics := newMCPMetrics()
	metrics.init(sink)
	metrics.recordDiscover(t.Context(), InitializeAttrs{ClientName: "modern-client", ProtocolVersion: "2026-07-28"})
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.counts) != 1 || sink.counts[0].name != "mcp.discover" || sink.counts[0].value != 1 {
		t.Fatalf("discover metric = %+v", sink.counts)
	}
	assertStringAttr(t, sink.counts[0].attrs, AttrMCPProtocolVersion, "2026-07-28")
}

func TestMCPRequestClientAttrsWithoutParams(t *testing.T) {
	t.Parallel()
	attrs := MCPRequestClientAttrs(&mcp.ServerRequest[*mcp.ListToolsParams]{})
	if attrs.ProtocolVersion != "" || attrs.ClientName != "" {
		t.Fatalf("metadata unexpectedly populated: %+v", attrs)
	}
}

type recordingMetricSink struct {
	mu     sync.Mutex
	counts []countRecord
}

var _ MetricSink = (*recordingMetricSink)(nil)

type countRecord struct {
	name  string
	value int64
	attrs []attribute.KeyValue
}

func (s *recordingMetricSink) Count(_ context.Context, name string, value int64, attrs ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts = append(s.counts, countRecord{name: name, value: value, attrs: slices.Clone(attrs)})
}

func (s *recordingMetricSink) Distribution(context.Context, string, float64, string, ...attribute.KeyValue) {
}
