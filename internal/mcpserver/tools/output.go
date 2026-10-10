package tools

import (
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsiteapi"
)

type sourceOutput struct {
	BaseURL     string `json:"baseUrl"`
	UpstreamURL string `json:"upstreamUrl,omitempty"`
}

type errorOutput struct {
	StatusCode int                 `json:"statusCode,omitempty"`
	Status     string              `json:"status,omitempty"`
	Code       *int                `json:"code,omitempty"`
	Message    string              `json:"message"`
	Fixes      []string            `json:"fixes,omitempty"`
	Candidates []pkgsite.Candidate `json:"candidates,omitempty"`
	RetryAfter string              `json:"retryAfter,omitempty"`
}

type outputMetadata struct {
	Source sourceOutput `json:"source"`
	Error  *errorOutput `json:"error,omitempty"`
}

type paginationOutput struct {
	FetchedItems          int    `json:"fetchedItems" jsonschema:"Number of items fetched in the current upstream page."`
	DisplayedItems        int    `json:"displayedItems"`
	StartAt               int    `json:"startAt"`
	NextStartAt           *int   `json:"nextStartAt" jsonschema:"Pass as start_at to continue displaying the current fetched page; null when exhausted."`
	Total                 *int   `json:"total,omitempty" jsonschema:"Upstream total when available; otherwise the number of fetched items."`
	UpstreamNextPageToken string `json:"upstreamNextPageToken" jsonschema:"Pass as token to fetch another upstream page; empty when exhausted."`
}

type listOutput[T any] struct {
	Items      []T               `json:"items"`
	Pagination *paginationOutput `json:"pagination,omitempty"`
}

type moduleOutput struct {
	pkgsiteapi.Module
	outputMetadata
}

type packageOutput struct {
	pkgsiteapi.Package
	ImportCount *int `json:"importCount,omitempty" jsonschema:"Number of returned imports; omitted unless imports were included."`
	outputMetadata
}

type searchOutput struct {
	Query  string `json:"query"`
	Symbol string `json:"symbol,omitempty"`
	listOutput[pkgsiteapi.SearchResult]
	outputMetadata
}

type versionsOutput struct {
	ModulePath string `json:"modulePath,omitempty"`
	listOutput[pkgsiteapi.ModuleVersion]
	outputMetadata
}

type packagesOutput struct {
	ModulePath        string `json:"modulePath,omitempty"`
	Version           string `json:"version,omitempty"`
	IsStandardLibrary bool   `json:"isStandardLibrary"`
	listOutput[pkgsiteapi.PackageInfo]
	outputMetadata
}

type packageContext struct {
	PackagePath string `json:"packagePath,omitempty"`
	ModulePath  string `json:"modulePath,omitempty"`
	Version     string `json:"version,omitempty"`
}

type symbolsOutput struct {
	packageContext
	listOutput[pkgsiteapi.Symbol]
	outputMetadata
}

type importedByItem struct {
	Path string `json:"path"`
}

type importedByOutput struct {
	packageContext
	listOutput[importedByItem]
	outputMetadata
}

type vulnsOutput struct {
	Path       string `json:"path,omitempty"`
	ModulePath string `json:"modulePath,omitempty"`
	Version    string `json:"version,omitempty"`
	listOutput[pkgsiteapi.Vulnerability]
	outputMetadata
}

func metadataFromResult(result pkgsite.Result) outputMetadata {
	return outputMetadata{Source: sourceOutput{BaseURL: pkgsite.DefaultBaseURL, UpstreamURL: result.UpstreamURL}, Error: errorFromResult(result.Error)}
}

func errorFromResult(err *pkgsite.APIError) *errorOutput {
	if err == nil {
		return nil
	}
	return &errorOutput{StatusCode: err.StatusCode, Status: err.Status, Code: err.Code, Message: err.Message, Fixes: err.Fixes, Candidates: err.Candidates, RetryAfter: err.RetryAfter}
}

func moduleOutputFromResult(result pkgsite.Result) (moduleOutput, error) {
	module, err := decodeOutput[pkgsiteapi.Module](result.Raw)
	if module.Path == nil {
		module.Path = summaryPointer(result.Summary, "path")
	}
	if module.Version == nil {
		module.Version = summaryPointer(result.Summary, "version")
	}
	return moduleOutput{Module: module, outputMetadata: metadataFromResult(result)}, err
}

func packageOutputFromResult(result pkgsite.Result) (packageOutput, error) {
	pkg, err := decodeOutput[pkgsiteapi.Package](result.Raw)
	if pkg.Path == nil {
		pkg.Path = summaryPointer(result.Summary, "path")
	}
	if pkg.Version == nil {
		pkg.Version = summaryPointer(result.Summary, "version")
	}
	output := packageOutput{Package: pkg, outputMetadata: metadataFromResult(result)}
	if pkg.Imports != nil {
		count := len(*pkg.Imports)
		output.ImportCount = &count
	}
	return output, err
}

func summaryPointer(summary map[string]any, key string) *string {
	value := summaryString(summary, key)
	if value == "" {
		return nil
	}
	return &value
}

func contextFromResult(result pkgsite.Result) packageContext {
	return packageContext{PackagePath: summaryString(result.Summary, "packagePath"), ModulePath: summaryString(result.Summary, "modulePath"), Version: summaryString(result.Summary, "version")}
}

func searchOutputFromResult(result pkgsite.Result) searchOutput {
	return searchOutput{Query: summaryString(result.Summary, "query"), Symbol: summaryString(result.Summary, "symbol"), outputMetadata: metadataFromResult(result)}
}

func versionsOutputFromResult(result pkgsite.Result) versionsOutput {
	return versionsOutput{ModulePath: summaryString(result.Summary, "modulePath"), outputMetadata: metadataFromResult(result)}
}

func packagesOutputFromResult(result pkgsite.Result) packagesOutput {
	isStd, _ := result.Summary["isStandardLibrary"].(bool)
	return packagesOutput{ModulePath: summaryString(result.Summary, "modulePath"), Version: summaryString(result.Summary, "version"), IsStandardLibrary: isStd, outputMetadata: metadataFromResult(result)}
}

func symbolsOutputFromResult(result pkgsite.Result) symbolsOutput {
	return symbolsOutput{packageContext: contextFromResult(result), outputMetadata: metadataFromResult(result)}
}

func importedByOutputFromResult(result pkgsite.Result) importedByOutput {
	return importedByOutput{packageContext: contextFromResult(result), outputMetadata: metadataFromResult(result)}
}

func vulnsOutputFromResult(result pkgsite.Result) vulnsOutput {
	return vulnsOutput{Path: summaryString(result.Summary, "path"), ModulePath: summaryString(result.Summary, "modulePath"), Version: summaryString(result.Summary, "version"), outputMetadata: metadataFromResult(result)}
}
