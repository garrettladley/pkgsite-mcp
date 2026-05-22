---
name: pkgsite/entities
description: Definitions for the core pkg.go.dev API entities.
related: pkgsite/overview, pkgsite/precision
---

# Entities

A module is the versioned unit named by `module_path`, such as `golang.org/x/oauth2`.

A package is an import path, such as `golang.org/x/oauth2/clientcredentials`. Some package paths can be ambiguous across modules; pass `module_path` when ambiguity appears.

A symbol is an exported package API item returned by `pkgsite_symbols`, usually with a name, kind, parent, and synopsis.

A vulnerability is a pkg.go.dev vulnerability record for a module or package path.
