package observability

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var cacheMetrics = newCacheMetrics()

type CacheOutcome string

const (
	CacheOutcomeHit      CacheOutcome = "hit"
	CacheOutcomeMiss     CacheOutcome = "miss"
	CacheOutcomeDisabled CacheOutcome = "disabled"
	CacheOutcomeError    CacheOutcome = "error"
	CacheOutcomeBypass   CacheOutcome = "bypass"
)

type cacheMetricSet struct {
	sink atomic.Pointer[metricSinkHolder]

	lookups      metric.Int64Counter
	hits         metric.Int64Counter
	misses       metric.Int64Counter
	writes       metric.Int64Counter
	writeErrors  metric.Int64Counter
	lookupMillis metric.Float64Histogram
}

type metricSinkHolder struct {
	sink MetricSink
}

func newCacheMetrics() *cacheMetricSet {
	meter := Meter("cache")
	lookups, _ := meter.Int64Counter("pkgsite.cache.lookup.count")
	hits, _ := meter.Int64Counter("pkgsite.cache.hit.count")
	misses, _ := meter.Int64Counter("pkgsite.cache.miss.count")
	writes, _ := meter.Int64Counter("pkgsite.cache.write.count")
	writeErrors, _ := meter.Int64Counter("pkgsite.cache.write_error.count")
	lookupMillis, _ := meter.Float64Histogram("pkgsite.cache.lookup.duration", metric.WithUnit("ms"))
	return &cacheMetricSet{
		lookups:      lookups,
		hits:         hits,
		misses:       misses,
		writes:       writes,
		writeErrors:  writeErrors,
		lookupMillis: lookupMillis,
	}
}

func initCacheMetrics(sink MetricSink) {
	if sink == nil {
		cacheMetrics.sink.Store(nil)
		return
	}
	cacheMetrics.sink.Store(&metricSinkHolder{sink: sink})
}

func RecordCacheLookup(ctx context.Context, outcome CacheOutcome, duration time.Duration) {
	outcomeAttr := attribute.String("cache.outcome", string(outcome))
	cacheMetrics.lookups.Add(ctx, 1, metric.WithAttributes(outcomeAttr))
	cacheMetrics.lookupMillis.Record(ctx, float64(duration)/float64(time.Millisecond), metric.WithAttributes(outcomeAttr))
	switch outcome {
	case CacheOutcomeHit:
		cacheMetrics.hits.Add(ctx, 1)
	case CacheOutcomeMiss:
		cacheMetrics.misses.Add(ctx, 1)
	case CacheOutcomeDisabled, CacheOutcomeError, CacheOutcomeBypass:
	}

	if holder := cacheMetrics.sink.Load(); holder != nil {
		holder.sink.Count(ctx, "pkgsite.cache.lookup", 1, outcomeAttr)
		holder.sink.Distribution(ctx, "pkgsite.cache.lookup.duration", float64(duration)/float64(time.Millisecond), "ms", outcomeAttr)
	}
}

func RecordCacheWrite(ctx context.Context, ok bool) {
	outcome := "ok"
	if !ok {
		outcome = "error"
	}
	outcomeAttr := attribute.String("cache.write.outcome", outcome)
	cacheMetrics.writes.Add(ctx, 1, metric.WithAttributes(outcomeAttr))
	if !ok {
		cacheMetrics.writeErrors.Add(ctx, 1)
	}

	if holder := cacheMetrics.sink.Load(); holder != nil {
		holder.sink.Count(ctx, "pkgsite.cache.write", 1, outcomeAttr)
	}
}
