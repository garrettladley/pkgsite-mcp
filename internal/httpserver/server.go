package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver"
	"github.com/garrettladley/pkgsite-mcp/internal/middleware"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryobs "github.com/garrettladley/pkgsite-mcp/internal/observability/sentry"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Config struct {
	Addr          string
	KV            config.KV
	Observability config.Observability
	Pkgsite       config.Pkgsite
	RateLimit     config.RateLimit
	Sentry        config.Sentry
}

func ConfigFromEnv(addr string) (Config, error) {
	cfg, err := config.Read()
	if err != nil {
		return Config{}, err
	}
	return Config{Addr: cfg.HTTPAddr(addr), KV: cfg.KV, Observability: cfg.Observability, Pkgsite: cfg.Pkgsite, RateLimit: cfg.RateLimit, Sentry: cfg.Sentry}, nil
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	obs, err := observability.Setup(ctx, observabilityOptions(cfg.Observability), logger, sentryobs.New(cfg.Sentry.DSN))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := obs.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(ctx, "shutdown observability", slog.Any("error", err))
		}
	}()
	logger = obs.Logger

	store, err := kv.NewStore(cfg.KV.RedisURL)
	if err != nil {
		return fmt.Errorf("configure kv store: %w", err)
	}
	client, err := pkgsite.New(cfg.Pkgsite, store)
	if err != nil {
		return fmt.Errorf("initialize pkgsite client: %w", err)
	}

	mcpHandler := mcpserver.New(client).Handler()
	mux := http.NewServeMux()
	mux.Handle("POST /mcp", mcpHandler)
	mux.Handle("GET /mcp", mcpHandler)
	mux.Handle("DELETE /mcp", mcpHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`+"\n", version.Version)
	})

	handler := middleware.Chain(
		securityHeaders,
		obs.Middleware,
		logging(logger),
		recovery(obs),
		middleware.RateLimit(store, cfg.RateLimit, logger),
	)(mux)
	handler = otelhttp.NewHandler(handler, "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Method + " " + r.Pattern
			}
			return r.Method + " " + r.URL.Path
		}),
	)

	baseCtx, cancelBase := context.WithCancel(ctx)
	defer cancelBase()

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(done)

	errCh := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "starting pkgsite-mcp http server", slog.String("addr", cfg.Addr), slog.String("version", version.Version))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case sig := <-done:
		logger.InfoContext(ctx, "shutdown signal received", slog.String("signal", sig.String()))
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	cancelBase()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return <-errCh
}

func observabilityOptions(cfg config.Observability) observability.Options {
	return observability.Options{
		ServiceName:      cfg.ServiceName,
		ServiceVersion:   version.Version,
		Environment:      cfg.Environment,
		FlushTimeout:     cfg.FlushTimeout,
		TracesSampleRate: cfg.TracesSampleRate,
		EnableLogs:       cfg.EnableLogs,
		EnableMetrics:    cfg.EnableMetrics,
	}
}

func logging(logger *slog.Logger) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			args := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			}
			for _, attr := range observability.TraceAttrs(r.Context()) {
				args = append(args, attr)
			}
			logger.InfoContext(r.Context(), "http request", args...)
		})
	}
}

func recovery(obs *observability.Handle) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					obs.Recover(r.Context(), recovered)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
