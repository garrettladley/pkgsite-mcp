package observability

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	AttrMCPMethodName              = "mcp.method.name"
	AttrMCPTransport               = "mcp.transport"
	AttrMCPRequestID               = "mcp.request.id"
	AttrMCPSessionID               = "mcp.session.id"
	AttrMCPToolName                = "mcp.tool.name"
	AttrMCPPromptName              = "mcp.prompt.name"
	AttrMCPResourceURI             = "mcp.resource.uri"
	AttrMCPResourceName            = "mcp.resource.name"
	AttrMCPToolResultIsError       = "mcp.tool.result.is_error"
	AttrMCPToolResultContentCount  = "mcp.tool.result.content_count"
	AttrMCPToolResultContent       = "mcp.tool.result.content"
	AttrMCPNotificationDirection   = "mcp.notification.direction"
	AttrMCPResponseDisplayedItems  = "mcp.response.displayed_items"
	AttrMCPResponseTotalItems      = "mcp.response.total_items"
	AttrMCPResponseStartAt         = "mcp.response.start_at"
	AttrMCPResponseTruncated       = "mcp.response.truncated"
	AttrMCPResponseNextStartAt     = "mcp.response.next_start_at"
	AttrMCPResponseMaxTokens       = "mcp.response.max_tokens"
	AttrMCPResponseMaxTokensBucket = "mcp.response.max_tokens_bucket"
	AttrMCPClientName              = "mcp.client.name"
	AttrMCPClientTitle             = "mcp.client.title"
	AttrMCPClientVersion           = "mcp.client.version"
	AttrMCPServerName              = "mcp.server.name"
	AttrMCPServerTitle             = "mcp.server.title"
	AttrMCPServerVersion           = "mcp.server.version"
	AttrMCPProtocolVersion         = "mcp.protocol.version"
	AttrMCPProtocolVersionHeader   = "mcp.protocol.version_header"
	AttrMCPRequestArgumentPrefix   = "mcp.request.argument."

	AttrPkgsiteLookupKind         = "pkgsite.lookup.kind"
	AttrPkgsiteModulePath         = "pkgsite.module_path"
	AttrPkgsitePackagePath        = "pkgsite.package_path"
	AttrPkgsitePath               = "pkgsite.path"
	AttrPkgsiteVersionRequested   = "pkgsite.version.requested"
	AttrPkgsiteVersionResolved    = "pkgsite.version.resolved"
	AttrPkgsiteVersionClass       = "pkgsite.version.class"
	AttrPkgsiteQueryHasFilter     = "pkgsite.query.has_filter"
	AttrPkgsitePaginationHasToken = "pkgsite.pagination.has_token"
	AttrPkgsiteQueryLimit         = "pkgsite.query.limit"
	AttrPkgsiteResponseFromCache  = "pkgsite.response.from_cache"
	AttrPkgsiteUpstreamURLPresent = "pkgsite.upstream_url_present"
	AttrPkgsiteResultCount        = "pkgsite.result.count"
	AttrPkgsiteErrorStatusCode    = "pkgsite.error.status_code"
	AttrPkgsiteEndpoint           = "pkgsite.endpoint"
	AttrPkgsiteCacheOutcome       = "pkgsite.cache.outcome"
	AttrPkgsiteCacheKeyHash       = "pkgsite.cache.key_hash"
	AttrPkgsiteCacheTTLMS         = "pkgsite.cache.ttl_ms"
	AttrPkgsiteCacheWriteOutcome  = "pkgsite.cache.write.outcome"
	AttrPkgsiteWarmDrain          = "pkgsite.warm.drain"
	AttrPkgsiteWarmKind           = "pkgsite.warm.kind"
	AttrPkgsiteWarmPage           = "pkgsite.warm.page"
	AttrPkgsiteWarmQueueCount     = "pkgsite.warm.queue.count"
	AttrPkgsiteWarmOutcome        = "pkgsite.warm.outcome"

	AttrHTTPRequestMethod      = "http.request.method"
	AttrHTTPResponseStatusCode = "http.response.status_code"
	AttrClientAddress          = "client.address"
	AttrClientPort             = "client.port"
	AttrURLPath                = "url.path"
	AttrNetworkTransport       = "network.transport"
	AttrNetworkProtocolVersion = "network.protocol.version"

	AttrRateLimitOutcome         = "ratelimit.outcome"
	AttrRateLimitRemainingBucket = "ratelimit.remaining_bucket"
	AttrRateLimitLimit           = "ratelimit.limit"
	AttrRateLimitWindowMS        = "ratelimit.window_ms"
)

type PkgsiteEndpoint string

const (
	PkgsiteEndpointUnknown    PkgsiteEndpoint = "unknown"
	PkgsiteEndpointSearch     PkgsiteEndpoint = "search"
	PkgsiteEndpointModule     PkgsiteEndpoint = "module"
	PkgsiteEndpointPackage    PkgsiteEndpoint = "package"
	PkgsiteEndpointVersions   PkgsiteEndpoint = "versions"
	PkgsiteEndpointPackages   PkgsiteEndpoint = "packages"
	PkgsiteEndpointSymbols    PkgsiteEndpoint = "symbols"
	PkgsiteEndpointImportedBy PkgsiteEndpoint = "imported_by"
	PkgsiteEndpointVulns      PkgsiteEndpoint = "vulns"
)

type VersionClass string

const (
	VersionClassEmpty    VersionClass = "empty"
	VersionClassFloating VersionClass = "floating"
	VersionClassBranch   VersionClass = "branch"
	VersionClassPinned   VersionClass = "pinned"
	VersionClassOther    VersionClass = "other"
)

type CacheWriteOutcome string

const (
	CacheWriteOutcomeOK      CacheWriteOutcome = "ok"
	CacheWriteOutcomeError   CacheWriteOutcome = "error"
	CacheWriteOutcomeSkipped CacheWriteOutcome = "skipped"
)

type WarmOutcome string

const (
	WarmOutcomeScheduled      WarmOutcome = "scheduled"
	WarmOutcomeSuccess        WarmOutcome = "success"
	WarmOutcomeTransportError WarmOutcome = "transport_error"
	WarmOutcomeAPIError       WarmOutcome = "api_error"
	WarmOutcomeSkipped        WarmOutcome = "skipped"
	WarmOutcomeUnknownKind    WarmOutcome = "unknown_kind"
)

type RateLimitOutcome string

const (
	RateLimitOutcomeAllowed    RateLimitOutcome = "allowed"
	RateLimitOutcomeLimited    RateLimitOutcome = "limited"
	RateLimitOutcomeStoreError RateLimitOutcome = "store_error"
	RateLimitOutcomeCanceled   RateLimitOutcome = "canceled"
	RateLimitOutcomeDeadline   RateLimitOutcome = "deadline_exceeded"
	RateLimitOutcomeDisabled   RateLimitOutcome = "disabled"
	RateLimitOutcomeSkipped    RateLimitOutcome = "skipped"
)

type ToolAttrs struct {
	ToolName         string
	LookupKind       string
	ModulePath       string
	PackagePath      string
	Path             string
	VersionRequested string
	VersionResolved  string
	HasFilter        bool
	HasToken         bool
	Limit            int
}

func (a ToolAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrMCPToolName, a.ToolName),
	}
	if a.LookupKind != "" {
		attrs = append(attrs, attribute.String(AttrPkgsiteLookupKind, a.LookupKind))
	}
	attrs = appendTrimmedString(attrs, AttrPkgsiteModulePath, a.ModulePath)
	attrs = appendTrimmedString(attrs, AttrPkgsitePackagePath, a.PackagePath)
	attrs = appendTrimmedString(attrs, AttrPkgsitePath, a.Path)
	attrs = appendTrimmedString(attrs, AttrPkgsiteVersionRequested, a.VersionRequested)
	attrs = appendTrimmedString(attrs, AttrPkgsiteVersionResolved, a.VersionResolved)
	attrs = append(attrs, attribute.String(AttrPkgsiteVersionClass, string(ClassifyVersion(a.VersionRequested))))
	if a.HasFilter {
		attrs = append(attrs, attribute.Bool(AttrPkgsiteQueryHasFilter, true))
	}
	if a.HasToken {
		attrs = append(attrs, attribute.Bool(AttrPkgsitePaginationHasToken, true))
	}
	if a.Limit > 0 {
		attrs = append(attrs, attribute.Int(AttrPkgsiteQueryLimit, a.Limit))
	}
	return attrs
}

type ResultAttrs struct {
	FromCache          bool
	UpstreamURLPresent bool
	ResultCount        int
	ErrorStatusCode    int
}

func (a ResultAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Bool(AttrPkgsiteResponseFromCache, a.FromCache),
		attribute.Bool(AttrPkgsiteUpstreamURLPresent, a.UpstreamURLPresent),
		attribute.Int(AttrPkgsiteResultCount, a.ResultCount),
	}
	if a.ErrorStatusCode > 0 {
		attrs = append(attrs, attribute.Int(AttrPkgsiteErrorStatusCode, a.ErrorStatusCode))
	}
	return attrs
}

type EnvelopeAttrs struct {
	DisplayedItems int
	TotalItems     int
	StartAt        int
	NextStartAt    int
	MaxTokens      int
	Truncated      bool
}

func (a EnvelopeAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Int(AttrMCPResponseDisplayedItems, a.DisplayedItems),
		attribute.Int(AttrMCPResponseTotalItems, a.TotalItems),
		attribute.Int(AttrMCPResponseStartAt, a.StartAt),
		attribute.Bool(AttrMCPResponseTruncated, a.Truncated),
	}
	if a.NextStartAt > 0 {
		attrs = append(attrs, attribute.Int(AttrMCPResponseNextStartAt, a.NextStartAt))
	}
	if a.MaxTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrMCPResponseMaxTokens, a.MaxTokens))
		attrs = append(attrs, attribute.String(AttrMCPResponseMaxTokensBucket, bucketMaxTokens(a.MaxTokens)))
	}
	return attrs
}

type InitializeAttrs struct {
	ClientName            string
	ClientTitle           string
	ClientVersion         string
	ProtocolVersion       string
	ProtocolVersionHeader string
}

func (a InitializeAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrMCPClientName, metricString(a.ClientName)),
		attribute.String(AttrMCPClientTitle, metricString(a.ClientTitle)),
		attribute.String(AttrMCPClientVersion, metricString(a.ClientVersion)),
		attribute.String(AttrMCPProtocolVersion, metricString(a.ProtocolVersion)),
	}
	attrs = appendTrimmedString(attrs, AttrMCPProtocolVersionHeader, a.ProtocolVersionHeader)
	return attrs
}

type CacheLookupAttrs struct {
	Method       string
	URL          *url.URL
	Outcome      CacheOutcome
	KeyHash      string
	TTL          time.Duration
	StatusCode   int
	WriteOutcome CacheWriteOutcome
}

func (a CacheLookupAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrHTTPRequestMethod, a.Method),
	}
	if a.Outcome != "" {
		attrs = append(attrs, attribute.String(AttrPkgsiteCacheOutcome, string(a.Outcome)))
	}
	if a.URL != nil {
		attrs = append(attrs,
			attribute.String(AttrURLPath, a.URL.EscapedPath()),
			attribute.String(AttrPkgsiteEndpoint, string(EndpointFromURL(a.URL))),
			attribute.String(AttrPkgsiteVersionClass, string(ClassifyVersion(a.URL.Query().Get("version")))),
			attribute.Bool(AttrPkgsiteQueryHasFilter, strings.TrimSpace(a.URL.Query().Get("filter")) != ""),
			attribute.Bool(AttrPkgsitePaginationHasToken, strings.TrimSpace(a.URL.Query().Get("token")) != ""),
		)
		if limit := intQueryValue(a.URL, "limit"); limit > 0 {
			attrs = append(attrs, attribute.Int(AttrPkgsiteQueryLimit, limit))
		}
	}
	attrs = appendTrimmedString(attrs, AttrPkgsiteCacheKeyHash, a.KeyHash)
	if a.TTL > 0 {
		attrs = append(attrs, attribute.Int64(AttrPkgsiteCacheTTLMS, a.TTL.Milliseconds()))
	}
	if a.StatusCode > 0 {
		attrs = append(attrs, attribute.Int(AttrHTTPResponseStatusCode, a.StatusCode))
	}
	if a.WriteOutcome != "" {
		attrs = append(attrs, attribute.String(AttrPkgsiteCacheWriteOutcome, string(a.WriteOutcome)))
	}
	return attrs
}

type WarmAttrs struct {
	Kind       string
	Drain      bool
	Page       int
	QueueCount int
	Outcome    WarmOutcome
}

func (a WarmAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Bool(AttrPkgsiteWarmDrain, a.Drain),
	}
	attrs = appendTrimmedString(attrs, AttrPkgsiteWarmKind, a.Kind)
	if a.Page > 0 {
		attrs = append(attrs, attribute.Int(AttrPkgsiteWarmPage, a.Page))
	}
	if a.QueueCount > 0 {
		attrs = append(attrs, attribute.Int(AttrPkgsiteWarmQueueCount, a.QueueCount))
	}
	if a.Outcome != "" {
		attrs = append(attrs, attribute.String(AttrPkgsiteWarmOutcome, string(a.Outcome)))
	}
	return attrs
}

type RateLimitAttrs struct {
	Outcome         RateLimitOutcome
	RemainingBucket string
	Limit           int
	Window          time.Duration
}

func (a RateLimitAttrs) Attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrRateLimitOutcome, string(a.Outcome)),
	}
	attrs = appendTrimmedString(attrs, AttrRateLimitRemainingBucket, a.RemainingBucket)
	if a.Limit > 0 {
		attrs = append(attrs, attribute.Int(AttrRateLimitLimit, a.Limit))
	}
	if a.Window > 0 {
		attrs = append(attrs, attribute.Int64(AttrRateLimitWindowMS, a.Window.Milliseconds()))
	}
	return attrs
}

func EndpointFromURL(u *url.URL) PkgsiteEndpoint {
	if u == nil {
		return PkgsiteEndpointUnknown
	}
	path := strings.TrimPrefix(u.EscapedPath(), "/")
	path = strings.TrimPrefix(path, "v1beta/")
	switch {
	case path == "search":
		return PkgsiteEndpointSearch
	case strings.HasPrefix(path, "module/"):
		return PkgsiteEndpointModule
	case strings.HasPrefix(path, "package/"):
		return PkgsiteEndpointPackage
	case strings.HasPrefix(path, "versions/"):
		return PkgsiteEndpointVersions
	case strings.HasPrefix(path, "packages/"):
		return PkgsiteEndpointPackages
	case strings.HasPrefix(path, "symbols/"):
		return PkgsiteEndpointSymbols
	case strings.HasPrefix(path, "imported-by/"):
		return PkgsiteEndpointImportedBy
	case strings.HasPrefix(path, "vuln/") || strings.HasPrefix(path, "vulns/"):
		return PkgsiteEndpointVulns
	default:
		return PkgsiteEndpointUnknown
	}
}

func ClassifyVersion(version string) VersionClass {
	switch strings.TrimSpace(version) {
	case "":
		return VersionClassEmpty
	case "latest":
		return VersionClassFloating
	case "main", "master":
		return VersionClassBranch
	default:
		if strings.HasPrefix(version, "v") && strings.Contains(version, ".") {
			return VersionClassPinned
		}
		return VersionClassOther
	}
}

func CacheWriteOutcomeFromOK(ok bool) CacheWriteOutcome {
	if ok {
		return CacheWriteOutcomeOK
	}
	return CacheWriteOutcomeError
}

func RemainingBucket(remaining int64, limit int) string {
	switch {
	case remaining <= 0:
		return "empty"
	case limit > 0 && remaining*4 <= int64(limit):
		return "low"
	default:
		return "ok"
	}
}

func appendTrimmedString(attrs []attribute.KeyValue, key, value string) []attribute.KeyValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return attrs
	}
	return append(attrs, attribute.String(key, value))
}

func metricString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func intQueryValue(u *url.URL, key string) int {
	if u == nil {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(u.Query().Get(key)))
	if err != nil {
		return 0
	}
	return value
}

func bucketMaxTokens(maxTokens int) string {
	switch {
	case maxTokens <= 1_000:
		return "lte_1k"
	case maxTokens <= 5_000:
		return "lte_5k"
	case maxTokens <= 10_000:
		return "lte_10k"
	default:
		return "gt_10k"
	}
}

func HTTPStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
