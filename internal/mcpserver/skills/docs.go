package skills

import (
	"fmt"
	"strings"

	_ "embed"
)

//go:embed docs/overview.md
var skillOverview string

//go:embed docs/entities.md
var skillEntities string

//go:embed docs/operations.md
var skillOperations string

//go:embed docs/pagination.md
var skillPagination string

//go:embed docs/precision.md
var skillPrecision string

type Name string

const (
	Overview   Name = "pkgsite/overview"
	Entities   Name = "pkgsite/entities"
	Operations Name = "pkgsite/operations"
	Pagination Name = "pkgsite/pagination"
	Precision  Name = "pkgsite/precision"
)

type Skill struct {
	Name        Name
	Description string
	Related     []Name
	Content     string
}

const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldRelated     = "related"
)

var (
	entitiesSkill = Skill{
		Name:        Entities,
		Description: "Definitions for the core pkg.go.dev API entities.",
		Related:     []Name{Overview, Precision},
		Content:     strings.TrimSpace(skillEntities),
	}
	operationsSkill = Skill{
		Name:        Operations,
		Description: "Operation-by-operation guidance for the MCP tools.",
		Related:     []Name{Pagination, Precision},
		Content:     strings.TrimSpace(skillOperations),
	}
	overviewSkill = Skill{
		Name:        Overview,
		Description: "What pkgsite-mcp is and when to use it.",
		Related:     []Name{Entities, Operations, Pagination, Precision},
		Content:     strings.TrimSpace(skillOverview),
	}
	paginationSkill = Skill{
		Name:        Pagination,
		Description: "How upstream page tokens and local response truncation work.",
		Related:     []Name{Operations},
		Content:     strings.TrimSpace(skillPagination),
	}
	precisionSkill = Skill{
		Name:        Precision,
		Description: "How to avoid module and package ambiguity.",
		Related:     []Name{Entities, Operations},
		Content:     strings.TrimSpace(skillPrecision),
	}

	catalog = []Skill{
		entitiesSkill,
		operationsSkill,
		overviewSkill,
		paginationSkill,
		precisionSkill,
	}
)

func List() ([]Skill, error) {
	skills := make([]Skill, len(catalog))
	copy(skills, catalog)
	return skills, nil
}

func Load(name string) (Skill, error) {
	switch Name(name) {
	case Entities:
		return entitiesSkill, nil
	case Operations:
		return operationsSkill, nil
	case Overview:
		return overviewSkill, nil
	case Pagination:
		return paginationSkill, nil
	case Precision:
		return precisionSkill, nil
	default:
		return Skill{}, fmt.Errorf("unknown pkgsite skill %q", name)
	}
}
