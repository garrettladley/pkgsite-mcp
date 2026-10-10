package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) symbols(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.SymbolsInput) (*mcp.CallToolResult, symbolsOutput, error) {
	if strings.TrimSpace(input.PackagePath) == "" {
		return nil, symbolsOutput{}, fmt.Errorf("package_path is required")
	}
	result, err := s.client.Symbols(ctx, input)
	res, err := s.result(ctx, result, toolNameSymbols, map[string]any{"package_path": input.PackagePath, "module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
	if err != nil {
		return nil, symbolsOutput{}, err
	}
	output := symbolsOutputFromResult(result)
	err = paginate(ctx, result, input.PageInput, &output, &output.listOutput)
	return res, output, err
}
