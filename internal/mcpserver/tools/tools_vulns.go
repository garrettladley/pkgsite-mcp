package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) vulns(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.VulnsInput) (*mcp.CallToolResult, vulnsOutput, error) {
	if strings.TrimSpace(input.Path) == "" {
		return nil, vulnsOutput{}, fmt.Errorf("path is required")
	}
	result, err := s.client.Vulns(ctx, input)
	res, err := s.result(ctx, result, toolNameVulns, map[string]any{"path": input.Path, "module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
	if err != nil {
		return nil, vulnsOutput{}, err
	}
	output := vulnsOutputFromResult(result)
	err = paginate(ctx, result, input.PageInput, &output, &output.listOutput)
	return res, output, err
}
