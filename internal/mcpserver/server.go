package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	sentryobs "github.com/garrettladley/pkgsite-mcp/internal/observability/sentry"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Server struct {
	client *pkgsite.Client
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

	client, err := pkgsite.New(cfg.Pkgsite)
	if err != nil {
		return err
	}
	return New(client).Run(ctx)
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

func New(client *pkgsite.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Run(ctx context.Context) error {
	server := s.mcpServer()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("run MCP server: %w", err)
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	server := s.mcpServer()
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:      true,
			JSONResponse:   true,
			SessionTimeout: 30 * time.Minute,
		},
	)
}

func (s *Server) mcpServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: version.Version}, nil)
	s.registerTools(server)
	return server
}

func (s *Server) registerTools(server *mcp.Server) {
	readOnly := true
	openWorld := true
	tool := func(name string) *mcp.Tool {
		return &mcp.Tool{Name: name, Description: ToolDescription(name), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &openWorld}}
	}
	mcp.AddTool(server, tool(toolNameListSkills), instrumentTool(toolNameListSkills, s.listSkills))
	mcp.AddTool(server, tool(toolNameLoadSkill), instrumentTool(toolNameLoadSkill, s.loadSkill))
	mcp.AddTool(server, tool(toolNameSearch), instrumentTool(toolNameSearch, s.search))
	mcp.AddTool(server, tool(toolNameModule), instrumentTool(toolNameModule, s.module))
	mcp.AddTool(server, tool(toolNamePackage), instrumentTool(toolNamePackage, s.packageInfo))
	mcp.AddTool(server, tool(toolNameVersions), instrumentTool(toolNameVersions, s.versions))
	mcp.AddTool(server, tool(toolNamePackages), instrumentTool(toolNamePackages, s.packages))
	mcp.AddTool(server, tool(toolNameSymbols), instrumentTool(toolNameSymbols, s.symbols))
	mcp.AddTool(server, tool(toolNameImportedBy), instrumentTool(toolNameImportedBy, s.importedBy))
	mcp.AddTool(server, tool(toolNameVulns), instrumentTool(toolNameVulns, s.vulns))
	mcp.AddTool(server, tool(toolNameExplain), instrumentTool(toolNameExplain, s.explain))
}

func instrumentTool[I any](toolName string, next func(context.Context, *mcp.CallToolRequest, I) (*mcp.CallToolResult, any, error)) func(context.Context, *mcp.CallToolRequest, I) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, any, error) {
		ctx, span := observability.Tracer("mcp").Start(ctx, "mcp.tool "+toolName, trace.WithAttributes(attribute.String("mcp.tool.name", toolName)))
		defer span.End()

		result, meta, err := next(ctx, req, input)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return result, meta, err
	}
}

func (s *Server) result(ctx context.Context, result pkgsite.Result, page pkgsite.PageInput, toolName string, nextArgs map[string]any, err error) (*mcp.CallToolResult, any, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("mcp.tool.name", toolName),
		attribute.Bool("pkgsite.from_cache", result.FromCache),
		attribute.Bool("pkgsite.upstream_url_present", result.UpstreamURL != ""),
		attribute.Int("pkgsite.result.count", len(result.Items)),
	)
	if err != nil {
		return nil, nil, err
	}
	if result.Error != nil {
		span.SetAttributes(attribute.Int("pkgsite.error.status_code", result.Error.StatusCode))
		span.SetStatus(codes.Error, result.Error.Status)
	}
	opts := envelopeOptions{Source: pkgsite.DefaultBaseURL, UpstreamURL: result.UpstreamURL, ToolName: toolName, NextArgs: nextArgs}
	if len(result.Items) > 0 {
		return textResult(paginatedEnvelope(result, page, opts)), nil, nil
	}
	return textResult(singleEnvelope(result, opts)), nil, nil
}
