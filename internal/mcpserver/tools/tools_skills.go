package tools

import (
	"context"
	"fmt"

	"github.com/garrettladley/pkgsite-mcp/internal/mcpserver/skills"
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

type skillHeader struct {
	Name        skills.Name   `json:"name"`
	Description string        `json:"description"`
	Related     []skills.Name `json:"related"`
}

type skillsOutput struct {
	listOutput[skillHeader]
	Source sourceOutput `json:"source"`
}

type skillOutput struct {
	skillHeader
	Resources []string     `json:"resources"`
	Source    sourceOutput `json:"source"`
}

func (s *service) listSkills(ctx context.Context, _ *mcp.CallToolRequest, input listSkillsInput) (*mcp.CallToolResult, skillsOutput, error) {
	allSkills, err := skills.List()
	if err != nil {
		return nil, skillsOutput{}, err
	}
	headers := make([]map[string]any, 0, len(allSkills))
	for _, skill := range allSkills {
		headers = append(headers, map[string]any{"name": skill.Name, "description": skill.Description, "related": skill.Related})
	}
	output := skillsOutput{Source: sourceOutput{BaseURL: "embedded_docs"}}
	err = paginate(ctx, pkgsite.Result{Items: headers}, input.PageInput, &output, &output.listOutput)
	return nil, output, err
}

func (s *service) loadSkill(_ context.Context, _ *mcp.CallToolRequest, input loadSkillInput) (*mcp.CallToolResult, skillOutput, error) {
	if input.ResourcePath != "" {
		return nil, skillOutput{}, fmt.Errorf("skill resources are not available yet")
	}
	skill, err := skills.Load(input.SkillName)
	if err != nil {
		return nil, skillOutput{}, err
	}
	output := skillOutput{
		skillHeader: skillHeader{Name: skill.Name, Description: skill.Description, Related: skill.Related},
		Resources:   []string{}, Source: sourceOutput{BaseURL: "embedded_docs"},
	}
	if input.HeaderOnly {
		return nil, output, nil
	}
	return textResult(skill.Content), output, nil
}
