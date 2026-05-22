package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) vulns(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.VulnsInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.Path) == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	result, err := s.client.Vulns(ctx, input)
	return s.result(result, input.PageInput, toolNameVulns, map[string]any{"path": input.Path, "module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
