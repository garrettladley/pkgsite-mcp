package tools

import (
	"context"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		ctx, span := observability.Tracer("mcp").Start(ctx, "mcp.tool "+toolName, trace.WithAttributes(observability.ToolAttrs{ToolName: toolName, LookupKind: lookupKind(toolName)}.Attributes()...))
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
	span.SetAttributes(toolAttrs(toolName, nextArgs, result).Attributes()...)
	if err != nil {
		return nil, nil, err
	}
	resultAttrs := observability.ResultAttrs{FromCache: result.FromCache, UpstreamURLPresent: result.UpstreamURL != "", ResultCount: len(result.Items)}
	if result.Error != nil {
		resultAttrs.ErrorStatusCode = result.Error.StatusCode
		span.SetStatus(codes.Error, result.Error.Status)
	}
	span.SetAttributes(resultAttrs.Attributes()...)
	opts := envelopeOptions{Source: pkgsite.DefaultBaseURL, UpstreamURL: result.UpstreamURL, ToolName: toolName, NextArgs: nextArgs}
	if len(result.Items) > 0 || result.Pagination != nil {
		span.SetAttributes(envelopeAttrs(result, page).Attributes()...)
		return textResult(paginatedEnvelope(result, page, opts)), nil, nil
	}
	span.SetAttributes(observability.EnvelopeAttrs{DisplayedItems: 1, TotalItems: 1, MaxTokens: page.MaxTokens}.Attributes()...)
	return textResult(singleEnvelope(result, opts)), nil, nil
}

func toolAttrs(toolName string, args map[string]any, result pkgsite.Result) observability.ToolAttrs {
	return observability.ToolAttrs{
		ToolName:         toolName,
		LookupKind:       lookupKind(toolName),
		ModulePath:       stringArg(args, "module_path"),
		PackagePath:      stringArg(args, "package_path"),
		Path:             stringArg(args, "path"),
		VersionRequested: stringArg(args, "version"),
		VersionResolved:  summaryString(result.Summary, "version"),
		HasFilter:        strings.TrimSpace(stringArg(args, "filter")) != "",
		HasToken:         strings.TrimSpace(stringArg(args, "token")) != "",
		Limit:            intArg(args, "limit"),
	}
}

func envelopeAttrs(result pkgsite.Result, page pkgsite.PageInput) observability.EnvelopeAttrs {
	total := len(result.Items)
	if result.Pagination != nil {
		if value, ok := result.Pagination["total"].(int); ok && value > 0 {
			total = value
		}
	}
	return observability.EnvelopeAttrs{
		DisplayedItems: len(result.Items),
		TotalItems:     total,
		StartAt:        page.StartAt,
		MaxTokens:      page.MaxTokens,
		Truncated:      page.StartAt > 0 || len(result.Items) < total,
	}
}

func lookupKind(toolName string) string {
	return strings.TrimPrefix(toolName, "pkgsite_")
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	value, _ := args[key].(int)
	return value
}

func summaryString(summary map[string]any, key string) string {
	if summary == nil {
		return ""
	}
	value, _ := summary[key].(string)
	return value
}
