package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) module(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.ModuleInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, nil, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Module(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNameModule, map[string]any{"module_path": input.ModulePath, "version": input.Version}, err)
}
