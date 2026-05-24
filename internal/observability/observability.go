package observability

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/garrettladley/pkgsite-mcp"

type Options struct {
	ServiceName      string
	ServiceVersion   string
	ServiceRevision  string
	Environment      string
	FlushTimeout     time.Duration
	TracesSampleRate float64
	EnableLogs       bool
	EnableMetrics    bool
}

type Backend interface {
	Start(context.Context, Options, *slog.Logger) (BackendHandle, error)
}

type BackendHandle interface {
	TraceExporter() sdktrace.SpanExporter
	LogHandler() slog.Handler
	MetricSink() MetricSink
	Middleware(http.Handler) http.Handler
	Recover(context.Context, any)
	Shutdown(context.Context) error
}

type MetricSink interface {
	Count(ctx context.Context, name string, value int64, attrs ...attribute.KeyValue)
	Distribution(ctx context.Context, name string, value float64, unit string, attrs ...attribute.KeyValue)
}

type Handle struct {
	Logger *slog.Logger

	backend BackendHandle
	tracer  *sdktrace.TracerProvider
}

func Setup(ctx context.Context, opts Options, logger *slog.Logger, backend Backend) (*Handle, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	if opts.FlushTimeout == 0 {
		opts.FlushTimeout = 2 * time.Second
	}
	if strings.TrimSpace(opts.ServiceName) == "" {
		opts.ServiceName = "pkgsite-mcp"
	}

	handle := &Handle{Logger: logger}
	var backendHandle BackendHandle
	if backend != nil {
		started, err := backend.Start(ctx, opts, logger)
		if err != nil {
			return nil, err
		}
		backendHandle = started
		handle.backend = started
	}

	attrs := []attribute.KeyValue{
		attribute.String("service.name", opts.ServiceName),
		attribute.String("service.version", opts.ServiceVersion),
		attribute.String("deployment.environment.name", opts.Environment),
	}
	if revision := strings.TrimSpace(opts.ServiceRevision); revision != "" && revision != "unknown" {
		attrs = append(attrs, attribute.String("vcs.ref.head.revision", revision))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes("", attrs...),
	)
	if err != nil {
		return nil, err
	}

	var tpOpts []sdktrace.TracerProviderOption
	if backendHandle != nil && backendHandle.TraceExporter() != nil {
		tpOpts = append(tpOpts, sdktrace.WithBatcher(backendHandle.TraceExporter()))
	}
	tpOpts = append(tpOpts, sdktrace.WithResource(res))
	handle.tracer = sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(handle.tracer)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if backendHandle != nil {
		if logHandler := backendHandle.LogHandler(); logHandler != nil {
			handle.Logger = slog.New(multiHandler{handlers: []slog.Handler{logger.Handler(), logHandler}})
		}
		initCacheMetrics(backendHandle.MetricSink())
		initMCPMetrics(backendHandle.MetricSink())
	} else {
		initCacheMetrics(nil)
		initMCPMetrics(nil)
	}

	return handle, nil
}

func (h *Handle) Middleware(next http.Handler) http.Handler {
	if h == nil || h.backend == nil {
		return next
	}
	return h.backend.Middleware(next)
}

func (h *Handle) Recover(ctx context.Context, recovered any) {
	if h == nil || h.backend == nil {
		return
	}
	h.backend.Recover(ctx, recovered)
}

func (h *Handle) Shutdown(ctx context.Context) error {
	if h == nil {
		return nil
	}
	var errs []error
	if h.tracer != nil {
		errs = append(errs, h.tracer.Shutdown(ctx))
	}
	if h.backend != nil {
		errs = append(errs, h.backend.Shutdown(ctx))
	}
	return errors.Join(errs...)
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(instrumentationName + "/" + name)
}

func Meter(name string) metric.Meter {
	return otel.Meter(instrumentationName + "/" + name)
}

func TraceAttrs(ctx context.Context) []slog.Attr {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String("trace_id", spanCtx.TraceID().String()),
		slog.String("span_id", spanCtx.SpanID().String()),
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

var _ slog.Handler = multiHandler{}

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			err = errors.Join(err, handler.Handle(ctx, record.Clone()))
		}
	}
	return err
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return multiHandler{handlers: handlers}
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return multiHandler{handlers: handlers}
}
