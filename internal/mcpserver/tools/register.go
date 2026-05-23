package tools

import (
	"context"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type service struct {
	client *pkgsite.Client
}

func Register(server *mcp.Server, client *pkgsite.Client) {
	s := &service{client: client}
	readOnly := true
	openWorld := true
	newTool := func(name string) *mcp.Tool {
		return &mcp.Tool{Name: name, Description: Description(name), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &openWorld}}
	}
	addTool(server, newTool(toolNameListSkills), s.listSkills)
	addTool(server, newTool(toolNameLoadSkill), s.loadSkill)
	addTool(server, newTool(toolNameSearch), s.search)
	addTool(server, newTool(toolNameModule), s.module)
	addTool(server, newTool(toolNamePackage), s.packageInfo)
	addTool(server, newTool(toolNameVersions), s.versions)
	addTool(server, newTool(toolNamePackages), s.packages)
	addTool(server, newTool(toolNameSymbols), s.symbols)
	addTool(server, newTool(toolNameImportedBy), s.importedBy)
	addTool(server, newTool(toolNameVulns), s.vulns)
	addTool(server, newTool(toolNameExplain), s.explain)
}

func addTool[I, O any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[I, O]) {
	mcp.AddTool(server, tool, instrumentTool(tool.Name, handler))
}

func instrumentTool[I, O any](toolName string, next mcp.ToolHandlerFor[I, O]) mcp.ToolHandlerFor[I, O] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, O, error) {
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

func (s *service) result(ctx context.Context, result pkgsite.Result, page pkgsite.PageInput, toolName string, nextArgs map[string]any, err error) (*mcp.CallToolResult, any, error) {
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
	if len(result.Items) > 0 || result.Pagination != nil {
		return textResult(paginatedEnvelope(result, page, opts)), nil, nil
	}
	return textResult(singleEnvelope(result, opts)), nil, nil
}
