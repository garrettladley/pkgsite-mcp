# pkgsite-mcp

`pkgsite-mcp` is a read-only MCP server for the official `pkg.go.dev/v1beta` API.

It gives coding agents structured, current Go ecosystem facts about modules, packages, versions, symbols, reverse imports, and vulnerabilities without scraping HTML or cloning repositories.

## Build

```sh
just build
```

The binary is written to `.cache/bin/pkgsite-mcp`.

## Run

```sh
.cache/bin/pkgsite-mcp serve --transport stdio
```

HTTP transport is also available:

```sh
.cache/bin/pkgsite-mcp serve --transport http --addr :8080
```

The streamable HTTP MCP endpoint is mounted at `/mcp`; health is available at `/health`.

## Local services

```sh
just up
```

This starts Redis with Docker Compose. Run the HTTP MCP server separately:

```sh
just serve-http
```

Use `just down` to stop Redis and `just logs` to follow Redis logs.

## MCP config

```json
{
  "mcpServers": {
    "pkgsite": {
      "command": "/absolute/path/to/pkgsite-mcp/.cache/bin/pkgsite-mcp",
      "args": ["serve", "--transport", "stdio"]
    }
  }
}
```

For HTTP-capable MCP clients, point them at:

```text
http://localhost:8080/mcp
```

## Configuration

```text
PKGSITE_BASE_URL=https://pkg.go.dev/v1beta
KV_REDIS_URL=redis://localhost:9736/0
PKGSITE_HTTP_TIMEOUT=10s
PKGSITE_CACHE_DISABLED=false
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=120
RATE_LIMIT_WINDOW=1m
SENTRY_DSN=
O11Y_SERVICE_NAME=pkgsite-mcp
O11Y_ENVIRONMENT=
O11Y_FLUSH_TIMEOUT=2s
O11Y_TRACES_SAMPLE_RATE=1.0
O11Y_ENABLE_LOGS=true
O11Y_ENABLE_METRICS=true
```

Redis is optional. Without `KV_REDIS_URL`, requests go directly to pkg.go.dev and IP rate limiting is disabled. When Redis is configured, it backs both pkg.go.dev response caching and fixed-window IP rate limiting for `/mcp`.
Sentry is optional. Without `SENTRY_DSN`, observability calls stay no-op.

## Tools

- `pkgsite_list_skills`
- `pkgsite_load_skill`
- `pkgsite_search`
- `pkgsite_module`
- `pkgsite_package`
- `pkgsite_versions`
- `pkgsite_packages`
- `pkgsite_symbols`
- `pkgsite_imported_by`
- `pkgsite_vulns`
- `pkgsite_explain`
