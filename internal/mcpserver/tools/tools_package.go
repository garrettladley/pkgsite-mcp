package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) packageInfo(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.PackageInput) (*mcp.CallToolResult, packageOutput, error) {
	if strings.TrimSpace(input.PackagePath) == "" {
		return nil, packageOutput{}, fmt.Errorf("package_path is required")
	}
	result, err := s.client.Package(ctx, input)
	res, err := s.result(ctx, result, toolNamePackage, map[string]any{"package_path": input.PackagePath, "module_path": input.ModulePath, "version": input.Version}, err)
	if err != nil {
		return nil, packageOutput{}, err
	}
	output, err := packageOutputFromResult(result)
	return res, output, err
}
