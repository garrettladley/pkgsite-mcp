package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) module(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.ModuleInput) (*mcp.CallToolResult, moduleOutput, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, moduleOutput{}, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Module(ctx, input)
	res, err := s.result(ctx, result, toolNameModule, map[string]any{"module_path": input.ModulePath, "version": input.Version}, err)
	if err != nil {
		return nil, moduleOutput{}, err
	}
	output, err := moduleOutputFromResult(result)
	return res, output, err
}
