# Operations

Use `pkgsite_module` for module metadata, `pkgsite_versions` for available versions, and `pkgsite_packages` for package lists inside a module.

Use `pkgsite_package` for package documentation metadata and `pkgsite_symbols` for exported API facts. `pkgsite_symbols` is usually the highest-signal tool for coding agents.

Use `pkgsite_vulns` before making security-sensitive recommendations. Use `pkgsite_imported_by` sparingly because the result set can be large.

List operations accept an upstream `filter` written as a Go expression that returns a boolean. The supported subset is:

- values `true`, `false`, and `nil`;
- `==` and `!=` on any value;
- `+`, `-`, `*`, `/`, and `%` on integers;
- `+` on strings;
- `<`, `<=`, `>`, and `>=` on strings and integers; and
- parenthesized expressions.

The functions `contains(s, sub)`, `hasPrefix(s, pre)`, `hasSuffix(s, suf)`, and `matches(s, re)` are also available. `matches` takes a regular expression; a bare regular expression is not a valid filter. Each route exposes its JSON fields as variables, so examples include `name == "main"` for packages, `kind == "Type"` for symbols, and `hasPrefix(version, "v2.")` for versions. Pass the expression as plain text; the client percent-encodes query parameters.
