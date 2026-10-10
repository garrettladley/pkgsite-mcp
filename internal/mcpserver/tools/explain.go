package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsiteapi"
)

const (
	explainStatusOK       = "ok"
	explainStatusError    = "error"
	explainStatusNotFound = "not_found"
	explainStatusSkipped  = "skipped"
)

var majorVersionPathSegment = regexp.MustCompile(`^v[2-9][0-9]*$`)

type explainPayload struct {
	Summary explainSummary `json:"summary"`
	Lookups explainLookups `json:"lookups"`
	Source  sourceOutput   `json:"source"`
}

type explainLookupOutput[T any] struct {
	Status string       `json:"status" jsonschema:"One of ok, error, not_found, or skipped."`
	Reason string       `json:"reason,omitempty"`
	Data   *T           `json:"data,omitempty"`
	Error  *errorOutput `json:"error,omitempty"`
}

type explainLookups struct {
	Module   explainLookupOutput[moduleOutput]   `json:"module"`
	Package  explainLookupOutput[packageOutput]  `json:"package"`
	Packages explainLookupOutput[packagesOutput] `json:"packages"`
	Symbols  explainLookupOutput[symbolsOutput]  `json:"symbols"`
	Vulns    explainLookupOutput[vulnsOutput]    `json:"vulns"`
}

type explainSummary struct {
	Path               string            `json:"path"`
	Version            string            `json:"version,omitempty"`
	Kind               string            `json:"kind"`
	ModulePath         string            `json:"modulePath,omitempty"`
	PackagePath        string            `json:"packagePath,omitempty"`
	ResolvedVersion    string            `json:"resolvedVersion,omitempty"`
	RepoURL            string            `json:"repoUrl,omitempty"`
	Name               string            `json:"name,omitempty"`
	Synopsis           string            `json:"synopsis,omitempty"`
	Goos               string            `json:"goos,omitempty"`
	Goarch             string            `json:"goarch,omitempty"`
	ImportCount        *int              `json:"importCount,omitempty"`
	IsLatest           bool              `json:"isLatest"`
	HasGoMod           bool              `json:"hasGoMod"`
	IsRedistributable  bool              `json:"isRedistributable"`
	IsStandardLibrary  bool              `json:"isStandardLibrary"`
	HasVulnerabilities bool              `json:"hasVulnerabilities"`
	Counts             map[string]int    `json:"counts"`
	KeySymbols         []string          `json:"keySymbols"`
	Statuses           map[string]string `json:"statuses"`
	Errors             map[string]string `json:"errors,omitempty"`
}

type explainParts struct {
	Module   explainSubResult
	Package  explainSubResult
	Packages explainSubResult
	Symbols  explainSubResult
	Vulns    explainSubResult
}

type explainSubResult struct {
	Status string         `json:"status"`
	Error  string         `json:"error,omitempty"`
	Result pkgsite.Result `json:"result"`
}

func buildExplainSummary(input pkgsite.ExplainInput, parts explainParts) explainSummary {
	statuses := map[string]string{
		"module":   parts.Module.Status,
		"package":  parts.Package.Status,
		"packages": parts.Packages.Status,
		"symbols":  parts.Symbols.Status,
		"vulns":    parts.Vulns.Status,
	}
	errors := explainErrors(map[string]explainSubResult{
		"module":   parts.Module,
		"package":  parts.Package,
		"packages": parts.Packages,
		"symbols":  parts.Symbols,
		"vulns":    parts.Vulns,
	})
	counts := map[string]int{
		"packages": len(parts.Packages.Result.Items),
		"symbols":  len(parts.Symbols.Result.Items),
		"vulns":    len(parts.Vulns.Result.Items),
	}

	summary := explainSummary{
		Path:               input.Path,
		Version:            input.Version,
		Kind:               explainKind(parts),
		Counts:             counts,
		KeySymbols:         keySymbolNames(parts.Symbols.Result.Items, 12),
		HasVulnerabilities: counts["vulns"] > 0,
		Statuses:           statuses,
		Errors:             errors,
	}
	applyPackageSummary(&summary, parts.Package.Result.Summary)
	if pkg, ok := parts.Package.Result.Raw.(*pkgsiteapi.Package); ok && pkg.Imports != nil {
		count := len(*pkg.Imports)
		summary.ImportCount = &count
	}
	applyModuleSummary(&summary, parts.Module.Result.Summary)
	applyPackagesSummary(&summary, parts.Packages.Result.Summary)
	applySymbolsSummary(&summary, parts.Symbols.Result.Summary)
	applyVulnsSummary(&summary, parts.Vulns.Result.Summary)
	if summary.ModulePath == "" {
		summary.ModulePath = input.ModulePath
	}
	if summary.PackagePath == "" && resultOK(parts.Package.Result, nil) {
		summary.PackagePath = input.Path
	}

	return summary
}

func explainSubResultFromResult(result pkgsite.Result, err error) explainSubResult {
	if err != nil {
		return explainSubResult{Status: explainStatusError, Error: err.Error()}
	}
	if result.Error != nil {
		status := explainStatusError
		if result.Error.StatusCode == 404 {
			status = explainStatusNotFound
		}
		return explainSubResult{Status: status, Error: result.Error.Message, Result: result}
	}
	return explainSubResult{Status: explainStatusOK, Result: result}
}

func explainSubResultSkipped(reason string) explainSubResult {
	return explainSubResult{Status: explainStatusSkipped, Error: reason}
}

func resultOK(result pkgsite.Result, err error) bool {
	return err == nil && result.Error == nil && (len(result.Summary) > 0 || len(result.Items) > 0 || result.Raw != nil)
}

func looksModuleLike(path string) bool {
	path = strings.Trim(path, "/ ")
	if path == "" {
		return false
	}
	if path == "std" || !strings.Contains(path, "/") {
		return true
	}
	parts := strings.Split(path, "/")
	if !strings.Contains(parts[0], ".") {
		return false
	}
	if len(parts) <= 3 {
		return true
	}
	return len(parts) == 4 && majorVersionPathSegment.MatchString(parts[3])
}

func explainKind(parts explainParts) string {
	switch {
	case parts.Package.Status == explainStatusOK:
		return "package"
	case parts.Module.Status == explainStatusOK:
		return "module"
	default:
		return "unknown"
	}
}

func explainErrors(results map[string]explainSubResult) map[string]string {
	errors := map[string]string{}
	for name, result := range results {
		if result.Status == explainStatusOK || result.Status == explainStatusSkipped || result.Error == "" {
			continue
		}
		errors[name] = result.Error
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

func applyModuleSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.ModulePath, data["path"])
	setString(&summary.ResolvedVersion, data["version"])
	setString(&summary.RepoURL, data["repoUrl"])
	setBool(&summary.IsLatest, data["isLatest"])
	setBool(&summary.HasGoMod, data["hasGoMod"])
	setBool(&summary.IsRedistributable, data["isRedistributable"])
	setBool(&summary.IsStandardLibrary, data["isStandardLibrary"])
}

func applyPackageSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.PackagePath, data["path"])
	setString(&summary.ModulePath, data["modulePath"])
	setString(&summary.ResolvedVersion, data["version"])
	setString(&summary.Name, data["name"])
	setString(&summary.Synopsis, data["synopsis"])
	setString(&summary.Goos, data["goos"])
	setString(&summary.Goarch, data["goarch"])
	setBool(&summary.IsLatest, data["isLatest"])
	setBool(&summary.IsRedistributable, data["isRedistributable"])
	setBool(&summary.IsStandardLibrary, data["isStandardLibrary"])
}

func applyPackagesSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.ModulePath, data["modulePath"])
	setString(&summary.ResolvedVersion, data["version"])
	setBool(&summary.IsStandardLibrary, data["isStandardLibrary"])
}

func applySymbolsSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.ModulePath, data["modulePath"])
	setString(&summary.ResolvedVersion, data["version"])
}

func applyVulnsSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.ModulePath, data["modulePath"])
	setString(&summary.ResolvedVersion, data["version"])
}

func keySymbolNames(items []map[string]any, limit int) []string {
	if limit <= 0 {
		return nil
	}
	names := make([]string, 0, min(limit, len(items)))
	seen := map[string]bool{}
	for _, item := range items {
		name := symbolName(item)
		if name == "" || seen[name] {
			continue
		}
		names = append(names, name)
		seen[name] = true
		if len(names) == limit {
			break
		}
	}
	return names
}

func symbolName(item map[string]any) string {
	for _, key := range []string{"name", "Name", "symbol", "Symbol"} {
		if value, ok := item[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func setString(target *string, value any) {
	if s, ok := value.(string); ok && s != "" {
		*target = s
	}
}

func setBool(target *bool, value any) {
	if b, ok := value.(bool); ok {
		*target = b
	}
}

func buildExplainPayload(ctx context.Context, input pkgsite.ExplainInput, parts explainParts) (explainPayload, error) {
	output := explainPayload{Summary: buildExplainSummary(input, parts), Source: sourceOutput{BaseURL: pkgsite.DefaultBaseURL}}
	page := input.PageInput
	page.MaxTokens = max(defaultMaxTokens, input.MaxTokens)
	module, err := moduleOutputFromResult(parts.Module.Result)
	if err != nil {
		return output, err
	}
	output.Lookups.Module = lookupOutput(parts.Module, module)
	pkg, err := packageOutputFromResult(parts.Package.Result)
	if err != nil {
		return output, err
	}
	output.Lookups.Package = lookupOutput(parts.Package, pkg)
	packages := packagesOutputFromResult(parts.Packages.Result)
	if err := paginate(ctx, parts.Packages.Result, page, &packages, &packages.listOutput); err != nil {
		return output, err
	}
	output.Lookups.Packages = lookupOutput(parts.Packages, packages)
	symbols := symbolsOutputFromResult(parts.Symbols.Result)
	if err := paginate(ctx, parts.Symbols.Result, page, &symbols, &symbols.listOutput); err != nil {
		return output, err
	}
	output.Lookups.Symbols = lookupOutput(parts.Symbols, symbols)
	vulns := vulnsOutputFromResult(parts.Vulns.Result)
	if err := paginate(ctx, parts.Vulns.Result, page, &vulns, &vulns.listOutput); err != nil {
		return output, err
	}
	output.Lookups.Vulns = lookupOutput(parts.Vulns, vulns)
	return output, fitExplain(&output, input.MaxTokens)
}

func lookupOutput[T any](part explainSubResult, data T) explainLookupOutput[T] {
	output := explainLookupOutput[T]{Status: part.Status}
	switch part.Status {
	case explainStatusOK:
		output.Data = &data
	case explainStatusSkipped:
		output.Reason = part.Error
	default:
		output.Error = errorFromResult(part.Result.Error)
		if output.Error == nil {
			output.Error = &errorOutput{Message: part.Error}
		}
	}
	return output
}

// fitExplain budgets the entire composed response, retaining at least one item
// per nonempty lookup. Its per-lookup pagination points into the fetched page.
func fitExplain(output *explainPayload, maxTokens int) error {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	for {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("encode explain output: %w", err)
		}
		if (len(data)+3)/4 <= maxTokens {
			return nil
		}
		// Trim the largest removable item so smaller lookups remain useful.
		largest := 0
		var trim func()
		if lookup := output.Lookups.Packages.Data; lookup != nil {
			size, remove := removableItem(&lookup.listOutput)
			if size > largest {
				largest, trim = size, remove
			}
		}
		if lookup := output.Lookups.Symbols.Data; lookup != nil {
			size, remove := removableItem(&lookup.listOutput)
			if size > largest {
				largest, trim = size, remove
			}
		}
		if lookup := output.Lookups.Vulns.Data; lookup != nil {
			size, remove := removableItem(&lookup.listOutput)
			if size > largest {
				trim = remove
			}
		}
		if trim == nil {
			return fmt.Errorf("max_tokens=%d is too small for this response; increase max_tokens or use the individual lookup tools", maxTokens)
		}
		trim()
	}
}

func removableItem[T any](list *listOutput[T]) (int, func()) {
	if len(list.Items) <= 1 {
		return 0, nil
	}
	data, err := json.Marshal(list.Items[len(list.Items)-1])
	if err != nil {
		return 0, nil
	}
	return len(data), func() {
		list.Items = list.Items[:len(list.Items)-1]
		list.Pagination.DisplayedItems = len(list.Items)
		next := list.Pagination.StartAt + len(list.Items)
		list.Pagination.NextStartAt = &next
	}
}
