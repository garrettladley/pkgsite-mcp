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
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver/skills"
	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver/tools"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryobs "github.com/garrettladley/pkgsite-mcp/internal/observability/sentry"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
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

	store, err := kvredis.New(cfg.KV)
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
		&mcp.ServerOptions{
			Logger: s.logger,
			Capabilities: &mcp.ServerCapabilities{Extensions: map[string]any{
				skills.Extension: map[string]any{},
			}},
			Instructions: "Use pkgsite for current Go package facts. Skill guides are available through the Skills extension, starting at skill://pkgsite/overview/SKILL.md. Clients without Skills support can use pkgsite_list_skills and pkgsite_load_skill.",
		},
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
	server.AddReceivingMiddleware(recordDiscoveryMetrics)
	tools.Register(server, s.client)
	if err := skills.Register(server); err != nil {
		panic(fmt.Sprintf("register embedded skills: %v", err))
	}
	return server
}

func recordDiscoveryMetrics(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if err == nil {
			switch method {
			case "initialize":
				observability.RecordMCPInitialize(ctx, observability.MCPRequestClientAttrs(req))
			case "server/discover":
				observability.RecordMCPDiscover(ctx, observability.MCPRequestClientAttrs(req))
			}
		}
		return result, err
	}
}
