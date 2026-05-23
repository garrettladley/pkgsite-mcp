package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type explainLookup struct {
	result pkgsite.Result
	err    error
}

func (s *service) explain(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.ExplainInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.Path) == "" {
		return nil, nil, fmt.Errorf("path is required")
	}

	var (
		moduleLookup    explainLookup
		packageLookup   explainLookup
		packagesLookup  explainLookup
		symbolsLookup   explainLookup
		vulnsLookup     explainLookup
		packagesSkipped string
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		moduleLookup.result, moduleLookup.err = s.client.Module(groupCtx, pkgsite.ModuleInput{ModulePath: input.Path, Version: input.Version})
		if input.IncludePackages || looksModuleLike(input.Path) || resultOK(moduleLookup.result, moduleLookup.err) {
			packagesLookup.result, packagesLookup.err = s.client.Packages(groupCtx, pkgsite.PackagesInput{ModulePath: input.Path, Version: input.Version, Limit: 25})
		} else {
			packagesSkipped = "path did not look module-like and module lookup did not succeed"
		}
		return nil
	})
	group.Go(func() error {
		packageLookup.result, packageLookup.err = s.client.Package(groupCtx, pkgsite.PackageInput{PackagePath: input.Path, ModulePath: input.ModulePath, Version: input.Version})
		return nil
	})
	group.Go(func() error {
		symbolsLookup.result, symbolsLookup.err = s.client.Symbols(groupCtx, pkgsite.SymbolsInput{PackagePath: input.Path, ModulePath: input.ModulePath, Version: input.Version, Limit: 50})
		return nil
	})
	group.Go(func() error {
		vulnsLookup.result, vulnsLookup.err = s.client.Vulns(groupCtx, pkgsite.VulnsInput{Path: input.Path, ModulePath: input.ModulePath, Version: input.Version})
		return nil
	})
	if err := group.Wait(); err != nil {
		return nil, nil, err
	}

	parts := explainParts{
		Module:  explainSubResultFromResult(moduleLookup.result, moduleLookup.err),
		Package: explainSubResultFromResult(packageLookup.result, packageLookup.err),
		Symbols: explainSubResultFromResult(symbolsLookup.result, symbolsLookup.err),
		Vulns:   explainSubResultFromResult(vulnsLookup.result, vulnsLookup.err),
	}
	if packagesSkipped == "" {
		parts.Packages = explainSubResultFromResult(packagesLookup.result, packagesLookup.err)
	} else {
		parts.Packages = explainSubResultSkipped(packagesSkipped)
	}

	data := buildExplainPayload(input, parts)
	trace.SpanFromContext(ctx).SetAttributes(observability.ToolAttrs{ToolName: toolNameExplain, LookupKind: "explain", Path: input.Path, ModulePath: input.ModulePath, VersionRequested: input.Version}.Attributes()...)
	return textResult(singleEnvelope(data, envelopeOptions{Source: pkgsite.DefaultBaseURL, ToolName: toolNameExplain})), nil, nil
}
