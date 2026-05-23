package tools

import (
	"regexp"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite"
)

const (
	explainStatusOK       = "ok"
	explainStatusError    = "error"
	explainStatusNotFound = "not_found"
	explainStatusSkipped  = "skipped"
)

var majorVersionPathSegment = regexp.MustCompile(`^v[2-9][0-9]*$`)

type explainPayload struct {
	Summary    explainSummary    `json:"summary"`
	SubResults explainSubResults `json:"subResults"`
}

type explainSummary struct {
	Path               string            `json:"path"`
	Version            string            `json:"version,omitempty"`
	Kind               string            `json:"kind"`
	ModulePath         string            `json:"modulePath,omitempty"`
	PackagePath        string            `json:"packagePath,omitempty"`
	ResolvedVersion    string            `json:"resolvedVersion,omitempty"`
	IsLatest           bool              `json:"isLatest"`
	IsStandardLibrary  bool              `json:"isStandardLibrary"`
	HasVulnerabilities bool              `json:"hasVulnerabilities"`
	Counts             map[string]int    `json:"counts"`
	KeySymbols         []string          `json:"keySymbols"`
	Statuses           map[string]string `json:"statuses"`
	Errors             map[string]string `json:"errors,omitempty"`
}

type explainSubResults struct {
	Module   explainSubResult `json:"module"`
	Package  explainSubResult `json:"package"`
	Packages explainSubResult `json:"packages"`
	Symbols  explainSubResult `json:"symbols"`
	Vulns    explainSubResult `json:"vulns"`
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

func buildExplainPayload(input pkgsite.ExplainInput, parts explainParts) explainPayload {
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

	return explainPayload{
		Summary:    summary,
		SubResults: explainSubResults(parts),
	}
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
	if len(result.Items) > 0 || result.Pagination != nil {
		result.Raw = nil
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
	setBool(&summary.IsLatest, data["isLatest"])
	setBool(&summary.IsStandardLibrary, data["isStandardLibrary"])
}

func applyPackageSummary(summary *explainSummary, data map[string]any) {
	if len(data) == 0 {
		return
	}
	setString(&summary.PackagePath, data["path"])
	setString(&summary.ModulePath, data["modulePath"])
	setString(&summary.ResolvedVersion, data["version"])
	setBool(&summary.IsLatest, data["isLatest"])
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
