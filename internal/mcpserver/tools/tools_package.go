package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) packageInfo(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.PackageInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.PackagePath) == "" {
		return nil, nil, fmt.Errorf("package_path is required")
	}
	result, err := s.client.Package(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNamePackage, map[string]any{"package_path": input.PackagePath, "module_path": input.ModulePath, "version": input.Version}, err)
}
