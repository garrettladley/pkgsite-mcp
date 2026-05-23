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

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
)

type Config struct {
	Addr    string
	Pkgsite config.Pkgsite
	Sentry  config.Sentry
}

func ConfigFromEnv(addr string) (Config, error) {
	cfg, err := config.Read()
	if err != nil {
		return Config{}, err
	}
	return Config{Addr: cfg.HTTPAddr(addr), Pkgsite: cfg.Pkgsite, Sentry: cfg.Sentry}, nil
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	client, err := pkgsite.New(cfg.Pkgsite)
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

	handler, flushSentry, err := handlerWithSentry(mux, cfg.Sentry, logger)
	if err != nil {
		return err
	}
	defer flushSentry()
	handler = securityHeaders(logging(logger, recovery(handler)))

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

func handlerWithSentry(next http.Handler, cfg config.Sentry, logger *slog.Logger) (http.Handler, func(), error) {
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		return next, func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:     dsn,
		Release: version.Version,
	}); err != nil {
		return nil, nil, fmt.Errorf("initialize sentry: %w", err)
	}
	logger.Info("sentry initialized")
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic:         true,
		WaitForDelivery: false,
	})
	return sentryHandler.Handle(next), func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
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
