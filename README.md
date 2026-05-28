# pkgsite-mcp

MCP tools for looking up current Go module and package information from the
official `pkg.go.dev/v1beta` API.

Use it when you want a coding agent to answer Go dependency questions with
structured pkg.go.dev data instead of guessing from model memory, scraping HTML,
or cloning repositories.

## Hosted server

MCP endpoint:

```text
https://pkgsite-mcp.fly.dev/mcp
```

The hosted server is read-only and does not require an API key.

## Add to Codex

Codex supports streamable HTTP MCP servers with `codex mcp add --url`:

```sh
codex mcp add pkgsite --url https://pkgsite-mcp.fly.dev/mcp
```

Equivalent `~/.codex/config.toml` entry:

```toml
[mcp_servers.pkgsite]
url = "https://pkgsite-mcp.fly.dev/mcp"
```

## Add to Claude Code

Claude Code supports HTTP MCP servers with `claude mcp add --transport http`:

```sh
claude mcp add --transport http pkgsite https://pkgsite-mcp.fly.dev/mcp
```

Use `--scope user` if you want it available outside the current project:

```sh
claude mcp add --scope user --transport http pkgsite https://pkgsite-mcp.fly.dev/mcp
```

Equivalent JSON shape:

```json
{
  "mcpServers": {
    "pkgsite": {
      "type": "http",
      "url": "https://pkgsite-mcp.fly.dev/mcp"
    }
  }
}
```

## Add to Cursor

Cursor supports URL-based MCP servers in `mcp.json`. Add this to your Cursor MCP
configuration:

```json
{
  "mcpServers": {
    "pkgsite": {
      "url": "https://pkgsite-mcp.fly.dev/mcp"
    }
  }
}
```

Cursor also accepts an explicit HTTP type in contexts that use its typed MCP
server config:

```json
{
  "mcpServers": {
    "pkgsite": {
      "type": "http",
      "url": "https://pkgsite-mcp.fly.dev/mcp"
    }
  }
}
```

## What agents can ask

`pkgsite-mcp` exposes read-only tools for common Go package research:

- Search packages and symbols on pkg.go.dev.
- Fetch module metadata, README text, licenses, versions, and contained packages.
- Fetch package metadata and exported symbols.
- Check which packages import a package.
- Check vulnerabilities for a module or package path.
- Load short built-in guides that help agents use the tools precisely.
- Run `pkgsite_explain` for a compact combined lookup of a module or package.

Examples of useful prompts after adding the MCP server:

```text
Use pkgsite to check the latest github.com/jackc/pgx/v5 version and summarize notable package symbols.
```

```text
Use pkgsite to inspect vulnerabilities for golang.org/x/crypto and tell me whether this repo should upgrade.
```

```text
Use pkgsite to compare the exported APIs of github.com/redis/go-redis/v9 packages relevant to clients and pipelines.
```

## Local development

Build the binary:

```sh
just build
```

Run over stdio:

```sh
.cache/bin/pkgsite-mcp serve --transport stdio
```

Run over streamable HTTP:

```sh
.cache/bin/pkgsite-mcp serve --transport http --addr :8080
```

Local MCP endpoint:

```text
http://localhost:8080/mcp
```

Health endpoint:

```text
http://localhost:8080/health
```

Start optional Redis for local caching and rate limiting:

```sh
just up
just serve-http
```

Use `just down` to stop Redis and `just logs` to follow Redis logs.

## Local MCP config

For clients that only support stdio, point them at a local binary:

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

For HTTP-capable clients, use:

```text
http://localhost:8080/mcp
```

## Configuration

```text
PKGSITE_BASE_URL=https://pkg.go.dev/v1beta
KV_REDIS_URL=redis://localhost:9736/0
KV_REDIS_POOL_SIZE=4
KV_REDIS_MIN_IDLE_CONNS=2
KV_REDIS_MAX_IDLE_CONNS=4
KV_REDIS_MAX_ACTIVE_CONNS=8
KV_REDIS_POOL_TIMEOUT=250ms
KV_REDIS_DIAL_TIMEOUT=1s
KV_REDIS_READ_TIMEOUT=750ms
KV_REDIS_WRITE_TIMEOUT=750ms
KV_REDIS_CONN_MAX_IDLE_TIME=10m
KV_REDIS_DISABLE_IDENTITY=true
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

Redis is optional. Without `KV_REDIS_URL`, requests go directly to pkg.go.dev and
IP rate limiting is disabled. When Redis is configured, it backs both pkg.go.dev
response caching and fixed-window IP rate limiting for `/mcp`.

Sentry is optional. Without `SENTRY_DSN`, observability calls stay no-op.
