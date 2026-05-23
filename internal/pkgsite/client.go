package pkgsite

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsite/transport"
	"github.com/garrettladley/pkgsite-mcp/internal/pkgsiteapi"
	"github.com/garrettladley/pkgsite-mcp/internal/version"
)

type Client struct {
	api    *pkgsiteapi.ClientWithResponses
	warmer Warmer
}

type Option func(*Client)

func WithWarmer(warmer Warmer) Option {
	return func(c *Client) {
		c.warmer = warmer
	}
}

func New(cfg config.Pkgsite, store kv.Store, opts ...Option) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	timeout := cfg.HTTPTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	doer := transport.NewCachedDoer(transport.NewHTTPClient(timeout), store, cfg.CacheDisabled)
	api, err := pkgsiteapi.NewClientWithResponses(
		baseURL,
		pkgsiteapi.WithHTTPClient(doer),
		pkgsiteapi.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("User-Agent", "pkgsite-mcp/"+version.Version)
			req.Header.Set("Accept", "application/json")
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	client := &Client{api: api}
	for _, opt := range opts {
		opt(client)
	}
	if client.warmer == nil && store != nil && !cfg.CacheDisabled {
		client.warmer = NewAsyncWarmer(client, AsyncWarmerOptions{})
	}
	return client, nil
}

func (c *Client) Search(ctx context.Context, input SearchInput) (Result, error) {
	params := &pkgsiteapi.GetSearchParams{
		Q:      new(input.Query),
		Symbol: optionalString(input.Symbol),
		Limit:  optionalInt(input.Limit),
		Token:  optionalString(input.Token),
		Filter: optionalString(input.Filter),
	}
	resp, err := c.api.GetSearchWithResponse(ctx, params)
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200)
	result := Result{
		Summary:     map[string]any{"query": input.Query, "symbol": input.Symbol, "count": len(items)},
		Items:       items,
		Pagination:  pagination(resp.JSON200, len(items)),
		Raw:         resp.JSON200,
		UpstreamURL: requestURL(resp.HTTPResponse),
		FromCache:   fromCache(resp.HTTPResponse),
	}
	c.warmSearchResult(ctx, result)
	return result, nil
}

func (c *Client) Module(ctx context.Context, input ModuleInput) (Result, error) {
	params := &pkgsiteapi.GetModuleParams{
		Version:  optionalString(input.Version),
		Readme:   optionalBool(input.IncludeReadme),
		Licenses: optionalBool(input.IncludeLicenses),
	}
	resp, err := c.api.GetModuleWithResponse(ctx, input.ModulePath, params)
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	module := resp.JSON200
	result := Result{
		Summary: map[string]any{
			"kind": "module", "path": stringVal(module.Path, input.ModulePath), "version": stringVal(module.Version, input.Version),
			"isLatest": boolVal(module.IsLatest), "latest": boolVal(module.IsLatest), "repoUrl": stringVal(module.RepoUrl, ""),
			"hasGoMod": boolVal(module.HasGoMod), "isRedistributable": boolVal(module.IsRedistributable), "isStandardLibrary": boolVal(module.IsStandardLibrary),
		},
		Raw: module, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse),
	}
	c.warm(ctx, WarmJob{Kind: WarmPackages, Packages: PackagesInput{ModulePath: stringValue(result.Summary["path"], input.ModulePath), Version: stringValue(result.Summary["version"], input.Version)}, Drain: true})
	return result, nil
}

func (c *Client) Package(ctx context.Context, input PackageInput) (Result, error) {
	params := &pkgsiteapi.GetPackageParams{
		Module: optionalString(input.ModulePath), Version: optionalString(input.Version), Goos: optionalString(input.Goos),
		Goarch: optionalString(input.Goarch), Doc: optionalString(input.DocFormat), Examples: optionalBool(input.IncludeExamples),
		Imports: optionalBool(input.IncludeImports), Licenses: optionalBool(input.IncludeLicenses),
	}
	resp, err := c.api.GetPackageWithResponse(ctx, input.PackagePath, params)
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	pkg := resp.JSON200
	result := Result{
		Summary: map[string]any{
			"kind": "package", "path": input.PackagePath, "modulePath": stringVal(pkg.ModulePath, input.ModulePath),
			"version": stringVal(pkg.Version, input.Version), "goos": stringVal(pkg.Goos, input.Goos), "goarch": stringVal(pkg.Goarch, input.Goarch),
			"isLatest": boolVal(pkg.IsLatest), "isStandardLibrary": boolVal(pkg.IsStandardLibrary), "importCount": lenStringSlice(pkg.Imports),
		},
		Raw: pkg, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse),
	}
	c.warm(ctx, WarmJob{Kind: WarmSymbols, Symbols: SymbolsInput{PackagePath: stringValue(result.Summary["path"], input.PackagePath), ModulePath: stringValue(result.Summary["modulePath"], input.ModulePath), Version: stringValue(result.Summary["version"], input.Version)}, Drain: true})
	return result, nil
}

func (c *Client) Versions(ctx context.Context, input VersionsInput) (Result, error) {
	resp, err := c.api.GetVersionsWithResponse(ctx, input.ModulePath, &pkgsiteapi.GetVersionsParams{
		Limit: optionalInt(input.Limit), Token: optionalString(input.Token), Filter: optionalString(input.Filter),
	})
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200)
	return Result{Summary: map[string]any{"kind": "versions", "modulePath": input.ModulePath, "count": len(items)}, Items: items, Pagination: pagination(resp.JSON200, len(items)), Raw: resp.JSON200, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse)}, nil
}

func (c *Client) Packages(ctx context.Context, input PackagesInput) (Result, error) {
	resp, err := c.api.GetPackagesWithResponse(ctx, input.ModulePath, &pkgsiteapi.GetPackagesParams{
		Version: optionalString(input.Version), Limit: optionalInt(input.Limit), Token: optionalString(input.Token), Filter: optionalString(input.Filter),
	})
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200.Packages)
	return Result{Summary: map[string]any{"kind": "module_packages", "modulePath": stringVal(resp.JSON200.ModulePath, input.ModulePath), "version": stringVal(resp.JSON200.Version, input.Version), "count": len(items), "isStandardLibrary": boolVal(resp.JSON200.IsStandardLibrary)}, Items: items, Pagination: pagination(resp.JSON200.Packages, len(items)), Raw: resp.JSON200, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse)}, nil
}

func (c *Client) Symbols(ctx context.Context, input SymbolsInput) (Result, error) {
	resp, err := c.api.GetSymbolsWithResponse(ctx, input.PackagePath, &pkgsiteapi.GetSymbolsParams{
		Module: optionalString(input.ModulePath), Version: optionalString(input.Version), Goos: optionalString(input.Goos), Goarch: optionalString(input.Goarch),
		Limit: optionalInt(input.Limit), Token: optionalString(input.Token), Filter: optionalString(input.Filter),
	})
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200.Symbols)
	return Result{Summary: map[string]any{"kind": "symbols", "packagePath": input.PackagePath, "modulePath": stringVal(resp.JSON200.ModulePath, input.ModulePath), "version": stringVal(resp.JSON200.Version, input.Version), "count": len(items)}, Items: items, Pagination: pagination(resp.JSON200.Symbols, len(items)), Raw: resp.JSON200, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse)}, nil
}

func (c *Client) ImportedBy(ctx context.Context, input ImportedByInput) (Result, error) {
	if input.Limit == 0 {
		input.Limit = 25
	}
	resp, err := c.api.GetImportedByWithResponse(ctx, input.PackagePath, &pkgsiteapi.GetImportedByParams{
		Module: optionalString(input.ModulePath), Version: optionalString(input.Version), Limit: optionalInt(input.Limit), Token: optionalString(input.Token), Filter: optionalString(input.Filter),
	})
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200.ImportedBy)
	return Result{Summary: map[string]any{"kind": "imported_by", "packagePath": input.PackagePath, "modulePath": stringVal(resp.JSON200.ModulePath, input.ModulePath), "version": stringVal(resp.JSON200.Version, input.Version), "count": len(items)}, Items: items, Pagination: pagination(resp.JSON200.ImportedBy, len(items)), Raw: resp.JSON200, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse)}, nil
}

func (c *Client) Vulns(ctx context.Context, input VulnsInput) (Result, error) {
	resp, err := c.api.GetVulnsWithResponse(ctx, input.Path, &pkgsiteapi.GetVulnsParams{
		Module: optionalString(input.ModulePath), Version: optionalString(input.Version), Limit: optionalInt(input.Limit), Token: optionalString(input.Token), Filter: optionalString(input.Filter),
	})
	if err != nil {
		return Result{}, err
	}
	if resp.JSON200 == nil {
		return resultError(resp.StatusCode(), resp.Status(), resp.Body, resp.HTTPResponse), nil
	}
	items := paginatedItems(resp.JSON200)
	return Result{Summary: map[string]any{"kind": "vulnerabilities", "path": input.Path, "modulePath": input.ModulePath, "version": input.Version, "count": len(items)}, Items: items, Pagination: pagination(resp.JSON200, len(items)), Raw: resp.JSON200, UpstreamURL: requestURL(resp.HTTPResponse), FromCache: fromCache(resp.HTTPResponse)}, nil
}

func (c *Client) warm(ctx context.Context, jobs ...WarmJob) {
	if c.warmer == nil || warmingDisabled(ctx) {
		return
	}
	c.warmer.Warm(ctx, jobs...)
}

func (c *Client) warmSearchResult(ctx context.Context, result Result) {
	if len(result.Items) != 1 {
		return
	}
	item := result.Items[0]
	path := stringValue(item["path"], "")
	if path == "" {
		return
	}
	c.warm(ctx, WarmJob{
		Kind: WarmPackage,
		Package: PackageInput{
			PackagePath: path,
			ModulePath:  stringValue(item["modulePath"], ""),
			Version:     stringValue(item["version"], ""),
		},
	})
}

func resultError(statusCode int, status string, body []byte, resp *http.Response) Result {
	var raw json.RawMessage
	if json.Valid(body) {
		raw = append(raw, body...)
	}
	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500]
	}
	return Result{Error: &APIError{StatusCode: statusCode, Status: status, Message: message, Body: raw}, UpstreamURL: requestURL(resp), FromCache: fromCache(resp)}
}

func paginatedItems(page *pkgsiteapi.PaginatedResponse) []map[string]any {
	if page == nil || page.Items == nil {
		return nil
	}
	return *page.Items
}

func pagination(page *pkgsiteapi.PaginatedResponse, count int) map[string]any {
	total := count
	next := ""
	if page != nil {
		if page.Total != nil {
			total = *page.Total
		}
		if page.NextPageToken != nil {
			next = *page.NextPageToken
		}
	}
	return map[string]any{"total": total, "displayedItems": count, "startAt": 0, "nextStartAt": nil, "upstreamNextPageToken": next}
}

func optionalString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func optionalInt(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func optionalBool(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}

func stringVal(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func stringValue(v any, fallback string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func boolVal(v *bool) bool {
	return v != nil && *v
}

func lenStringSlice(v *[]string) int {
	if v == nil {
		return 0
	}
	return len(*v)
}

func requestURL(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	return resp.Request.URL.String()
}

func fromCache(resp *http.Response) bool {
	return resp != nil && resp.Header.Get(transport.CacheHitHeader) == "true"
}
