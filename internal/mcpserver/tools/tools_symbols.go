package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) symbols(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.SymbolsInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.PackagePath) == "" {
		return nil, nil, fmt.Errorf("package_path is required")
	}
	result, err := s.client.Symbols(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNameSymbols, map[string]any{"package_path": input.PackagePath, "module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
