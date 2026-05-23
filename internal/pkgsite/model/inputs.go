package model

type PageInput struct {
	StartAt   int `json:"start_at,omitempty" jsonschema:"Zero-based local item offset for response truncation."`
	MaxTokens int `json:"max_tokens,omitempty" jsonschema:"Approximate maximum response tokens. Default 10000."`
}

type SearchInput struct {
	Query  string `json:"query" jsonschema:"Search query."`
	Symbol string `json:"symbol,omitempty" jsonschema:"Optional symbol search string."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Upstream max number of items."`
	Token  string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type ModuleInput struct {
	ModulePath      string `json:"module_path" jsonschema:"Go module path."`
	Version         string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	IncludeReadme   bool   `json:"include_readme,omitempty" jsonschema:"Include README contents."`
	IncludeLicenses bool   `json:"include_licenses,omitempty" jsonschema:"Include license contents."`
	PageInput
}

type PackageInput struct {
	PackagePath     string `json:"package_path" jsonschema:"Go package import path."`
	ModulePath      string `json:"module_path,omitempty" jsonschema:"Module path for disambiguation."`
	Version         string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	Goos            string `json:"goos,omitempty" jsonschema:"GOOS documentation context."`
	Goarch          string `json:"goarch,omitempty" jsonschema:"GOARCH documentation context."`
	DocFormat       string `json:"doc_format,omitempty" jsonschema:"Documentation format: text, html, md, markdown."`
	IncludeExamples bool   `json:"include_examples,omitempty" jsonschema:"Include examples with documentation."`
	IncludeImports  bool   `json:"include_imports,omitempty" jsonschema:"Include imports."`
	IncludeLicenses bool   `json:"include_licenses,omitempty" jsonschema:"Include licenses."`
	PageInput
}

type VersionsInput struct {
	ModulePath string `json:"module_path" jsonschema:"Go module path."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Upstream max number of items."`
	Token      string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter     string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type PackagesInput struct {
	ModulePath string `json:"module_path" jsonschema:"Go module path."`
	Version    string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Upstream max number of items."`
	Token      string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter     string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type SymbolsInput struct {
	PackagePath string `json:"package_path" jsonschema:"Go package import path."`
	ModulePath  string `json:"module_path,omitempty" jsonschema:"Module path for disambiguation."`
	Version     string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	Goos        string `json:"goos,omitempty" jsonschema:"GOOS documentation context."`
	Goarch      string `json:"goarch,omitempty" jsonschema:"GOARCH documentation context."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Upstream max number of items."`
	Token       string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter      string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type ImportedByInput struct {
	PackagePath string `json:"package_path" jsonschema:"Go package import path."`
	ModulePath  string `json:"module_path,omitempty" jsonschema:"Module path for disambiguation."`
	Version     string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Upstream max number of items. Defaults to 25."`
	Token       string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter      string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type VulnsInput struct {
	Path       string `json:"path" jsonschema:"Go module or package path."`
	ModulePath string `json:"module_path,omitempty" jsonschema:"Module path for disambiguation."`
	Version    string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Upstream max number of items."`
	Token      string `json:"token,omitempty" jsonschema:"Upstream page token."`
	Filter     string `json:"filter,omitempty" jsonschema:"Regular expression filter."`
	PageInput
}

type ExplainInput struct {
	Path            string `json:"path" jsonschema:"Go module or package path."`
	Version         string `json:"version,omitempty" jsonschema:"Version, latest, main, or master. Latest if omitted."`
	ModulePath      string `json:"module_path,omitempty" jsonschema:"Module path for package disambiguation."`
	IncludeSymbols  bool   `json:"include_symbols,omitempty" jsonschema:"Include symbols. Defaults to true."`
	IncludePackages bool   `json:"include_packages,omitempty" jsonschema:"Include module package list when path appears to be a module."`
	IncludeVulns    bool   `json:"include_vulns,omitempty" jsonschema:"Include vulnerabilities. Defaults to true."`
	PageInput
}
