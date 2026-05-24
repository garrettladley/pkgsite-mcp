package observability

import (
	"context"
	"slices"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestRecordMCPInitializeWritesMetricSink(t *testing.T) {
	sink := &recordingMetricSink{}
	initMCPMetrics(sink)
	t.Cleanup(func() { initMCPMetrics(nil) })

	RecordMCPInitialize(t.Context(), InitializeAttrs{
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
