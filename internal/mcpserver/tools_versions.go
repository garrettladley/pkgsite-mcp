package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) versions(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.VersionsInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.ModulePath) == "" {
		return nil, nil, fmt.Errorf("module_path is required")
	}
	result, err := s.client.Versions(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNameVersions, map[string]any{"module_path": input.ModulePath, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
