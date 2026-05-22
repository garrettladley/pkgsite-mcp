package mcpserver

import (
	"context"
	"fmt"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listSkillsInput struct {
	pkgsite.PageInput
}

type loadSkillInput struct {
	SkillName    string `json:"skill_name" jsonschema:"Exact skill name, for example pkgsite/overview."`
	HeaderOnly   bool   `json:"header_only,omitempty" jsonschema:"If true, return only summary and related skills."`
	ResourcePath string `json:"resource_path,omitempty" jsonschema:"Reserved for future bundled references."`
}

func (s *Server) listSkills(_ context.Context, _ *mcp.CallToolRequest, input listSkillsInput) (*mcp.CallToolResult, any, error) {
	skills, err := ListSkills()
	if err != nil {
		return nil, nil, err
	}
	headers := make([]map[string]any, 0, len(skills))
	for _, skill := range skills {
		headers = append(headers, map[string]any{skillFieldName: skill.Name, skillFieldDescription: skill.Description, skillFieldRelated: skill.Related})
	}
	return textResult(sliceEnvelope(headers, input.PageInput, envelopeOptions{Source: "embedded_docs", ToolName: toolNameListSkills})), nil, nil
}

func (s *Server) loadSkill(_ context.Context, _ *mcp.CallToolRequest, input loadSkillInput) (*mcp.CallToolResult, any, error) {
	if input.ResourcePath != "" {
		return nil, nil, fmt.Errorf("skill resources are not available yet")
	}
	skill, err := LoadSkill(input.SkillName)
	if err != nil {
		return nil, nil, err
	}
	if input.HeaderOnly {
		header := map[string]any{skillFieldName: skill.Name, skillFieldDescription: skill.Description, skillFieldRelated: skill.Related, "resources": []string{}}
		return textResult(singleEnvelope(header, envelopeOptions{Source: "embedded_docs"})), nil, nil
	}
	return textResult(skill.Content), nil, nil
}
