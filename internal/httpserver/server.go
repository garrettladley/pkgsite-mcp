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
	"strings"
	"syscall"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	kvredis "github.com/garrettladley/pkgsite-mcp/internal/kv/redis"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver"
	"github.com/garrettladley/pkgsite-mcp/internal/middleware"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryobs "github.com/garrettladley/pkgsite-mcp/internal/observability/sentry"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
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

	store, err := kvredis.New(cfg.KV.RedisURL)
	if err != nil {
		return fmt.Errorf("configure kv store: %w", err)
	}
	client, err := pkgsite.New(cfg.Pkgsite, store)
	if err != nil {
		return fmt.Errorf("initialize pkgsite client: %w", err)
	}

	mcpHandler := mcpserver.New(client, logger).Handler()
	mux := http.NewServeMux()
	mux.Handle("POST /mcp", mcpHandler)
	mux.Handle("GET /mcp", mcpHandler)
	mux.Handle("DELETE /mcp", mcpHandler)
	mux.HandleFunc("GET /health", health)

	handler := middleware.Chain(
		middleware.SecurityHeaders,
		obs.Middleware,
		middleware.Logging(logger),
		middleware.Recovery(obs),
		middleware.RateLimit(store, cfg.RateLimit, logger),
	)(mux)
	handler = otelhttp.NewHandler(handler, "http.server",
		otelhttp.WithSpanNameFormatter(httpSpanName),
	)
	handler = middleware.MCPRequestMetadata(handler)

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
		attrs := []any{slog.String("addr", cfg.Addr), slog.String("version", version.Release())}
		if commit := version.ShortCommit(); commit != "" {
			attrs = append(attrs, slog.String("commit", version.Commit))
		}
		logger.InfoContext(ctx, "starting pkgsite-mcp http server", attrs...)
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
		ServiceVersion:   version.Release(),
		ServiceRevision:  version.Commit,
		Environment:      cfg.Environment,
		FlushTimeout:     cfg.FlushTimeout,
		TracesSampleRate: cfg.TracesSampleRate,
		EnableLogs:       cfg.EnableLogs,
		EnableMetrics:    cfg.EnableMetrics,
	}
}

func httpSpanName(_ string, r *http.Request) string {
	name := r.Method + " " + r.URL.Path
	if r.Pattern != "" {
		if strings.HasPrefix(r.Pattern, r.Method+" ") {
			name = r.Pattern
		} else {
			name = r.Method + " " + r.Pattern
		}
	}
	if method := r.Header.Get(xhttp.HeaderInternalMCPMethod); method != "" {
		name += " " + method
		if mcpName := r.Header.Get(xhttp.HeaderInternalMCPName); mcpName != "" {
			name += " " + mcpName
		}
	}
	return name
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
}
