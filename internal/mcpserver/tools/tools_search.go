package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *service) search(ctx context.Context, _ *mcp.CallToolRequest, input pkgsite.SearchInput) (*mcp.CallToolResult, searchOutput, error) {
	if strings.TrimSpace(input.Query) == "" {
		return nil, searchOutput{}, fmt.Errorf("query is required")
	}
	result, err := s.client.Search(ctx, input)
	res, err := s.result(ctx, result, toolNameSearch, map[string]any{"query": input.Query, "symbol": input.Symbol, "limit": input.Limit, "token": input.Token, "filter": input.Filter}, err)
	if err != nil {
		return nil, searchOutput{}, err
	}
	output := searchOutputFromResult(result)
	err = paginate(ctx, result, input.PageInput, &output, &output.listOutput)
	return res, output, err
}
