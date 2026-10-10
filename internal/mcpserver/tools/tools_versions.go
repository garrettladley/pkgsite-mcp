package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) versions(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.VersionsInput) (*mcp.CallToolResult, versionsOutput, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, versionsOutput{}, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Versions(ctx, input)
	res, err := s.result(ctx, result, toolNameVersions, map[string]any{"module_path": input.ModulePath, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
	if err != nil {
		return nil, versionsOutput{}, err
	}
	output := versionsOutputFromResult(result)
	err = paginate(ctx, result, input.PageInput, &output, &output.listOutput)
	return res, output, err
}
