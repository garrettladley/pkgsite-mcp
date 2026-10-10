---
name: overview
description: What pkgsite-mcp is and when to use it.
metadata:
  related-skills: pkgsite/entities,pkgsite/operations,pkgsite/pagination,pkgsite/precision
---

# pkgsite-mcp overview

Use pkgsite-mcp when you need current structured facts from `pkg.go.dev/v1` about Go modules, packages, versions, exported symbols, imported-by relationships, or vulnerabilities.

The source of truth is pkg.go.dev. This server is read-only and does not clone repositories, scrape HTML, or infer facts from training data.

Prefer primitive tools when you know the exact operation. Use `pkgsite_explain` for quick exploration of a module or package path.

Data tools advertise an `outputSchema` and return a typed JSON object in `structuredContent`, with the same JSON in a text block for compatibility. Source URLs and pagination are ordinary JSON fields. Module and package metadata appear directly in the payload, without duplicate `summary` and `raw` copies.

Upstream API failures set MCP `isError: true` and include actionable `error` details. `pkgsite_explain` returns an aggregate `summary` and typed `lookups`, each with a `status` and either `data`, `error`, or a skip `reason`. It can succeed with partial lookup failures; it reports a tool error when neither a package nor a module can be resolved.

The `pkgsite_load_skill` compatibility tool returns the guide's Markdown body alongside its structured header. `header_only` returns the header as both structured content and JSON text. Native `resources/read` returns the complete `SKILL.md`, including YAML frontmatter.
