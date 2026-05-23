package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

func (s *service) explain(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.ExplainInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.Path) == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	moduleResult, moduleErr := s.client.Module(ctx, pkgsite.ModuleInput{ModulePath: input.Path, Version: input.Version})
	packageResult, packageErr := s.client.Package(ctx, pkgsite.PackageInput{PackagePath: input.Path, ModulePath: input.ModulePath, Version: input.Version})

	parts := explainParts{
		Module:  explainSubResultFromResult(moduleResult, moduleErr),
		Package: explainSubResultFromResult(packageResult, packageErr),
	}
	if input.IncludePackages || looksModuleLike(input.Path) || resultOK(moduleResult, moduleErr) {
		packages, err := s.client.Packages(ctx, pkgsite.PackagesInput{ModulePath: input.Path, Version: input.Version, Limit: 25})
		parts.Packages = explainSubResultFromResult(packages, err)
	} else {
		parts.Packages = explainSubResultSkipped("path did not look module-like and module lookup did not succeed")
	}
	symbols, err := s.client.Symbols(ctx, pkgsite.SymbolsInput{PackagePath: input.Path, ModulePath: input.ModulePath, Version: input.Version, Limit: 50})
	parts.Symbols = explainSubResultFromResult(symbols, err)
	vulns, err := s.client.Vulns(ctx, pkgsite.VulnsInput{Path: input.Path, ModulePath: input.ModulePath, Version: input.Version})
	parts.Vulns = explainSubResultFromResult(vulns, err)
	data := buildExplainPayload(input, parts)
	trace.SpanFromContext(ctx).SetAttributes(observability.ToolAttrs{ToolName: toolNameExplain, LookupKind: "explain", Path: input.Path, ModulePath: input.ModulePath, VersionRequested: input.Version}.Attributes()...)
	return textResult(singleEnvelope(data, envelopeOptions{Source: pkgsite.DefaultBaseURL, ToolName: toolNameExplain})), nil, nil
}
