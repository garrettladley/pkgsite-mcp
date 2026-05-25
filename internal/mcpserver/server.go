package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	kvredis "github.com/garrettladley/pkgsite-mcp/internal/kv/redis"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver/tools"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryobs "github.com/garrettladley/pkgsite-mcp/internal/observability/sentry"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpServerName       = "pkgsite"
	mcpServerTitle      = "pkg.go.dev MCP"
	mcpServerWebsiteURL = "https://github.com/garrettladley/pkgsite-mcp"

	// Stateless streamable HTTP sessions close when each request exits.
	statelessSessionTimeout time.Duration = 0
)

type Server struct {
	client *pkgsite.Client
	logger *slog.Logger
}

func RunStdio(ctx context.Context) error {
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	obs, err := observability.Setup(ctx, observabilityOptions(cfg.Observability), logger, sentryobs.New(cfg.Sentry.DSN))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := obs.Shutdown(shutdownCtx); err != nil {
			obs.Logger.ErrorContext(ctx, "shutdown observability", "error", err)
		}
	}()

	store, err := kvredis.New(cfg.KV.RedisURL)
	if err != nil {
		return fmt.Errorf("configure kv store: %w", err)
	}
	client, err := pkgsite.New(cfg.Pkgsite, store)
	if err != nil {
		return err
	}
	return New(client, obs.Logger).Run(ctx)
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

func New(client *pkgsite.Client, logger *slog.Logger) *Server {
	return &Server{client: client, logger: logger}
}

func (s *Server) Run(ctx context.Context) error {
	server := s.mcpServer(observability.MCPTransportStdio)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("run MCP server: %w", err)
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	server := s.mcpServer(observability.MCPTransportHTTP)
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:      true,
			JSONResponse:   true,
			Logger:         s.logger,
			SessionTimeout: statelessSessionTimeout,
		},
	)
}

func (s *Server) mcpServer(transport observability.MCPTransport) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:       mcpServerName,
			Title:      mcpServerTitle,
			Version:    version.Public(),
			WebsiteURL: mcpServerWebsiteURL,
		},
		&mcp.ServerOptions{Logger: s.logger},
	)
	mcpTracingConfig := observability.MCPServerTracingConfig{
		Transport: transport,
		Server: observability.MCPServerInfo{
			Name:    mcpServerName,
			Title:   mcpServerTitle,
			Version: version.Public(),
		},
	}
	server.AddReceivingMiddleware(observability.MCPServerTracingMiddleware(mcpTracingConfig))
	server.AddSendingMiddleware(observability.MCPServerSendingTracingMiddleware(mcpTracingConfig))
	server.AddReceivingMiddleware(recordInitializeMetrics)
	tools.Register(server, s.client)
	return server
}

func recordInitializeMetrics(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if method == "initialize" && err == nil {
			recordInitialize(ctx, req)
		}
		return result, err
	}
}

func recordInitialize(ctx context.Context, req mcp.Request) {
	params, ok := req.GetParams().(*mcp.InitializeParams)
	if !ok || params == nil {
		return
	}

	attrs := observability.InitializeAttrs{
		ProtocolVersion: params.ProtocolVersion,
	}
	if params.ClientInfo != nil {
		attrs.ClientName = params.ClientInfo.Name
		attrs.ClientTitle = params.ClientInfo.Title
		attrs.ClientVersion = params.ClientInfo.Version
	}
	if extra := req.GetExtra(); extra != nil {
		attrs.ProtocolVersionHeader = extra.Header.Get(xhttp.HeaderMCPProtocolVersion)
	}
	observability.RecordMCPInitialize(ctx, attrs)
}
