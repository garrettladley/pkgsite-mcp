package mcpserver

import (
	"fmt"
	"sort"
	"strings"

	_ "embed"
)

//go:embed docs/tools/pkgsite_list_skills.md
var toolListSkills string

//go:embed docs/tools/pkgsite_load_skill.md
var toolLoadSkill string

//go:embed docs/tools/pkgsite_search.md
var toolSearch string

//go:embed docs/tools/pkgsite_module.md
var toolModule string

//go:embed docs/tools/pkgsite_package.md
var toolPackage string

//go:embed docs/tools/pkgsite_versions.md
var toolVersions string

//go:embed docs/tools/pkgsite_packages.md
var toolPackages string

//go:embed docs/tools/pkgsite_symbols.md
var toolSymbols string

//go:embed docs/tools/pkgsite_imported_by.md
var toolImportedBy string

//go:embed docs/tools/pkgsite_vulns.md
var toolVulns string

//go:embed docs/tools/pkgsite_explain.md
var toolExplain string

//go:embed docs/skills/overview.md
var skillOverview string

//go:embed docs/skills/entities.md
var skillEntities string

//go:embed docs/skills/operations.md
var skillOperations string

//go:embed docs/skills/pagination.md
var skillPagination string

//go:embed docs/skills/precision.md
var skillPrecision string

type Skill struct {
	Name        string
	Description string
	Related     []string
	Content     string
}

const (
	mcpServerName = "pkgsite"

	toolNameListSkills = "pkgsite_list_skills"
	toolNameLoadSkill  = "pkgsite_load_skill"
	toolNameSearch     = "pkgsite_search"
	toolNameModule     = "pkgsite_module"
	toolNamePackage    = "pkgsite_package"
	toolNameVersions   = "pkgsite_versions"
	toolNamePackages   = "pkgsite_packages"
	toolNameSymbols    = "pkgsite_symbols"
	toolNameImportedBy = "pkgsite_imported_by"
	toolNameVulns      = "pkgsite_vulns"
	toolNameExplain    = "pkgsite_explain"

	skillFieldName        = "name"
	skillFieldDescription = "description"
	skillFieldRelated     = "related"
)

var toolDescriptions = map[string]string{
	toolNameListSkills: toolListSkills,
	toolNameLoadSkill:  toolLoadSkill,
	toolNameSearch:     toolSearch,
	toolNameModule:     toolModule,
	toolNamePackage:    toolPackage,
	toolNameVersions:   toolVersions,
	toolNamePackages:   toolPackages,
	toolNameSymbols:    toolSymbols,
	toolNameImportedBy: toolImportedBy,
	toolNameVulns:      toolVulns,
	toolNameExplain:    toolExplain,
}

var skillDocs = map[string]string{
	"pkgsite/overview":   skillOverview,
	"pkgsite/entities":   skillEntities,
	"pkgsite/operations": skillOperations,
	"pkgsite/pagination": skillPagination,
	"pkgsite/precision":  skillPrecision,
}

func ToolDescription(name string) string {
	description, ok := toolDescriptions[name]
	if !ok {
		panic(fmt.Sprintf("missing embedded MCP tool description %q", name))
	}
	return strings.TrimSpace(description)
}

func ListSkills() ([]Skill, error) {
	names := make([]string, 0, len(skillDocs))
	for name := range skillDocs {
		names = append(names, name)
	}
	sort.Strings(names)
	skills := make([]Skill, 0, len(names))
	for _, name := range names {
		skill, err := LoadSkill(name)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func LoadSkill(name string) (Skill, error) {
	data, ok := skillDocs[name]
	if !ok {
		return Skill{}, fmt.Errorf("unknown pkgsite skill %q", name)
	}
	return parseSkill(data, name), nil
}

func parseSkill(data string, fallbackName string) Skill {
	content := strings.TrimSpace(data)
	skill := Skill{Name: fallbackName, Content: content}
	if !strings.HasPrefix(content, "---\n") {
		return skill
	}
	rest := strings.TrimPrefix(content, "---\n")
	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return skill
	}
	skill.Content = strings.TrimSpace(after)
	for line := range strings.SplitSeq(before, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case skillFieldName:
			skill.Name = value
		case skillFieldDescription:
			skill.Description = value
		case skillFieldRelated:
			if value != "" {
				for related := range strings.SplitSeq(value, ",") {
					skill.Related = append(skill.Related, strings.TrimSpace(related))
				}
				sort.Strings(skill.Related)
			}
		}
	}
	return skill
}
