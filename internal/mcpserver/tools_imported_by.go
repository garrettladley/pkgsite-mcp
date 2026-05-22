package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) importedBy(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.ImportedByInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.PackagePath) == "" {
		return nil, nil, fmt.Errorf("package_path is required")
	}
	result, err := s.client.ImportedBy(ctx, input)
	return s.result(result, input.PageInput, toolNameImportedBy, map[string]any{"package_path": input.PackagePath, "module_path": input.ModulePath, "version": input.Version, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
