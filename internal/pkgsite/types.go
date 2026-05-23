package pkgsite

import "github.com/garrettladley/pkgsite-mcp/internal/pkgsite/model"

const DefaultBaseURL = model.DefaultBaseURL

type (
	APIError = model.APIError
	Result   = model.Result
)

type (
	PageInput       = model.PageInput
	SearchInput     = model.SearchInput
	ModuleInput     = model.ModuleInput
	PackageInput    = model.PackageInput
	VersionsInput   = model.VersionsInput
	PackagesInput   = model.PackagesInput
	SymbolsInput    = model.SymbolsInput
	ImportedByInput = model.ImportedByInput
	VulnsInput      = model.VulnsInput
	ExplainInput    = model.ExplainInput
)
