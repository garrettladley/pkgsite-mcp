package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	client *pkgsite.Client
}

func RunStdio(ctx context.Context) error {
	client, err := pkgsite.NewFromEnv()
	if err != nil {
		return err
	}
	return New(client).Run(ctx)
}

func New(client *pkgsite.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Run(ctx context.Context) error {
	server := s.mcpServer()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("run MCP server: %w", err)
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	server := s.mcpServer()
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:      true,
			JSONResponse:   true,
			SessionTimeout: 30 * time.Minute,
		},
	)
}

func (s *Server) mcpServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: version.Version}, nil)
	s.registerTools(server)
	return server
}

func (s *Server) registerTools(server *mcp.Server) {
	readOnly := true
	openWorld := true
	tool := func(name string) *mcp.Tool {
		return &mcp.Tool{Name: name, Description: ToolDescription(name), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &openWorld}}
	}
	mcp.AddTool(server, tool(toolNameListSkills), s.listSkills)
	mcp.AddTool(server, tool(toolNameLoadSkill), s.loadSkill)
	mcp.AddTool(server, tool(toolNameSearch), s.search)
	mcp.AddTool(server, tool(toolNameModule), s.module)
	mcp.AddTool(server, tool(toolNamePackage), s.packageInfo)
	mcp.AddTool(server, tool(toolNameVersions), s.versions)
	mcp.AddTool(server, tool(toolNamePackages), s.packages)
	mcp.AddTool(server, tool(toolNameSymbols), s.symbols)
	mcp.AddTool(server, tool(toolNameImportedBy), s.importedBy)
	mcp.AddTool(server, tool(toolNameVulns), s.vulns)
	mcp.AddTool(server, tool(toolNameExplain), s.explain)
}

func (s *Server) result(result pkgsite.Result, page pkgsite.PageInput, toolName string, nextArgs map[string]any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, err
	}
	opts := envelopeOptions{Source: pkgsite.DefaultBaseURL, UpstreamURL: result.UpstreamURL, ToolName: toolName, NextArgs: nextArgs}
	if len(result.Items) > 0 {
		return textResult(paginatedEnvelope(result, page, opts)), nil, nil
	}
	return textResult(singleEnvelope(result, opts)), nil, nil
}
