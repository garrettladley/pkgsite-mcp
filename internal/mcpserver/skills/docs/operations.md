# Operations

Use `pkgsite_module` for module metadata, `pkgsite_versions` for available versions, and `pkgsite_packages` for package lists inside a module.

Use `pkgsite_package` for package documentation metadata and `pkgsite_symbols` for exported API facts. `pkgsite_symbols` is usually the highest-signal tool for coding agents.

Use `pkgsite_vulns` before making security-sensitive recommendations. Use `pkgsite_imported_by` sparingly because the result set can be large.
