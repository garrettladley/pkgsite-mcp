---
name: pkgsite/overview
description: What pkgsite-mcp is and when to use it.
related: pkgsite/entities, pkgsite/operations, pkgsite/pagination, pkgsite/precision
---

# pkgsite-mcp overview

Use pkgsite-mcp when you need current structured facts from `pkg.go.dev/v1beta` about Go modules, packages, versions, exported symbols, imported-by relationships, or vulnerabilities.

The source of truth is pkg.go.dev. This server is read-only and does not clone repositories, scrape HTML, or infer facts from training data.

Prefer primitive tools when you know the exact operation. Use `pkgsite_explain` for quick exploration of a module or package path.
