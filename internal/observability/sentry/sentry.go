package sentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	getsentry "github.com/getsentry/sentry-go"
	sentryattribute "github.com/getsentry/sentry-go/attribute"
	sentryotel "github.com/getsentry/sentry-go/otel"
	sentryslog "github.com/getsentry/sentry-go/slog"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Backend struct {
	dsn string
}

var _ observability.Backend = Backend{}

func New(dsn string) observability.Backend {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil
	}
	return Backend{dsn: dsn}
}

func (b Backend) Start(ctx context.Context, opts observability.Options, _ *slog.Logger) (observability.BackendHandle, error) {
	if err := getsentry.Init(getsentry.ClientOptions{
		Dsn:              b.dsn,
		Release:          opts.ServiceVersion,
		Environment:      opts.Environment,
		EnableTracing:    true,
		TracesSampleRate: opts.TracesSampleRate,
		EnableLogs:       opts.EnableLogs,
		DisableMetrics:   !opts.EnableMetrics,
		Integrations: func(integrations []getsentry.Integration) []getsentry.Integration {
			return append(integrations, sentryotel.NewOtelIntegration())
		},
	}); err != nil {
		return nil, fmt.Errorf("initialize sentry: %w", err)
	}

	handle := &Handle{
		processors: []sdktrace.SpanProcessor{
			newMCPSpanProcessor(),
		},
		flushTimeout: opts.FlushTimeout,
	}
	if opts.EnableLogs {
		handle.logHandler = sentryslog.Option{
			LogLevel: []slog.Level{
				slog.LevelDebug,
				slog.LevelInfo,
				slog.LevelWarn,
				slog.LevelError,
				sentryslog.LevelFatal,
			},
			AddSource: true,
		}.NewSentryHandler(ctx)
	}
	if opts.EnableMetrics {
		meter := getsentry.NewMeter(ctx)
		attrs := []sentryattribute.Builder{
			sentryattribute.String("service.name", opts.ServiceName),
			sentryattribute.String("service.version", opts.ServiceVersion),
			sentryattribute.String("deployment.environment.name", opts.Environment),
		}
		if revision := strings.TrimSpace(opts.ServiceRevision); revision != "" && revision != "unknown" {
			attrs = append(attrs, sentryattribute.String("vcs.ref.head.revision", revision))
		}
		meter.SetAttributes(attrs...)
		handle.metricSink = metricSink{meter: meter}
	}
	return handle, nil
}

type Handle struct {
	processors   []sdktrace.SpanProcessor
	logHandler   slog.Handler
	metricSink   observability.MetricSink
	flushTimeout time.Duration
}

var (
	_ observability.BackendHandle = (*Handle)(nil)
	_ observability.MetricSink    = metricSink{}
)

func (h *Handle) TraceExporter() sdktrace.SpanExporter {
	return nil
}

func (h *Handle) TraceProcessors() []sdktrace.SpanProcessor {
	return h.processors
}

func (h *Handle) LogHandler() slog.Handler {
	return h.logHandler
}

func (h *Handle) MetricSink() observability.MetricSink {
	return h.metricSink
}

func (h *Handle) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := getsentry.GetHubFromContext(r.Context())
		if hub == nil {
			hub = getsentry.CurrentHub().Clone()
		}
		hub.Scope().SetRequest(r)
		next.ServeHTTP(w, r.WithContext(getsentry.SetHubOnContext(r.Context(), hub)))
	})
}

func (h *Handle) Recover(ctx context.Context, recovered any) {
	if hub := getsentry.GetHubFromContext(ctx); hub != nil {
		hub.RecoverWithContext(ctx, recovered)
	}
}

func (h *Handle) Shutdown(context.Context) error {
	if !getsentry.Flush(h.flushTimeout) {
		return errors.New("flush sentry: timeout")
	}
	return nil
}

type metricSink struct {
	meter getsentry.Meter
}

func (s metricSink) Count(ctx context.Context, name string, value int64, attrs ...attribute.KeyValue) {
	s.meter.WithCtx(ctx).Count(name, value, getsentry.WithAttributes(sentryAttrs(attrs)...))
}

func (s metricSink) Distribution(ctx context.Context, name string, value float64, unit string, attrs ...attribute.KeyValue) {
	s.meter.WithCtx(ctx).Distribution(name, value,
		getsentry.WithUnit(sentryUnit(unit)),
		getsentry.WithAttributes(sentryAttrs(attrs)...),
	)
}

func sentryAttrs(attrs []attribute.KeyValue) []sentryattribute.Builder {
	out := make([]sentryattribute.Builder, 0, len(attrs))
	for _, attr := range attrs {
		switch attr.Value.Type() {
		case attribute.BOOL:
			out = append(out, sentryattribute.Bool(string(attr.Key), attr.Value.AsBool()))
		case attribute.INT64:
			out = append(out, sentryattribute.Int64(string(attr.Key), attr.Value.AsInt64()))
		case attribute.FLOAT64:
			out = append(out, sentryattribute.Float64(string(attr.Key), attr.Value.AsFloat64()))
		default:
			out = append(out, sentryattribute.String(string(attr.Key), attr.Value.AsString()))
		}
	}
	return out
}

func sentryUnit(unit string) string {
	switch unit {
	case "ms":
		return getsentry.UnitMillisecond
	default:
		return unit
	}
}
