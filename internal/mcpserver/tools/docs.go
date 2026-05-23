package tools

import (
	"fmt"
	"strings"

	_ "embed"
)

//go:embed docs/pkgsite_list_skills.md
var toolListSkills string

//go:embed docs/pkgsite_load_skill.md
var toolLoadSkill string

//go:embed docs/pkgsite_search.md
var toolSearch string

//go:embed docs/pkgsite_module.md
var toolModule string

//go:embed docs/pkgsite_package.md
var toolPackage string

//go:embed docs/pkgsite_versions.md
var toolVersions string

//go:embed docs/pkgsite_packages.md
var toolPackages string

//go:embed docs/pkgsite_symbols.md
var toolSymbols string

//go:embed docs/pkgsite_imported_by.md
var toolImportedBy string

//go:embed docs/pkgsite_vulns.md
var toolVulns string

//go:embed docs/pkgsite_explain.md
var toolExplain string

const (
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
)

var descriptions = map[string]string{
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

func Description(name string) string {
	description, ok := descriptions[name]
	if !ok {
		panic(fmt.Sprintf("missing embedded MCP tool description %q", name))
	}
	return strings.TrimSpace(description)
}
