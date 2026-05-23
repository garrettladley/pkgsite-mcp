package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) search(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.SearchInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.Query) == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	result, err := s.client.Search(ctx, input)
	return s.result(ctx, result, input.PageInput, toolNameSearch, map[string]any{"query": input.Query, "symbol": input.Symbol, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
}
