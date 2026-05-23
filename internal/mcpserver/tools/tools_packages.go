package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) packages(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.PackagesInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, nil, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Packages(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNamePackages, map[string]any{"module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
