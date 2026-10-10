package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) packages(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.PackagesInput) (*mcp.CallToolResult, packagesOutput, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, packagesOutput{}, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Packages(ctx, input)
	res, err := s.result(ctx, result, toolNamePackages, map[string]any{"module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
	if err != nil {
		return nil, packagesOutput{}, err
	}
	output := packagesOutputFromResult(result)
	err = paginate(ctx, result, input.PageInput, &output, &output.listOutput)
	return res, output, err
}
