Load one embedded pkgsite skill guide by exact name, such as `pkgsite/overview`.

Compatibility access for clients without the MCP Skills extension. Skills-aware clients can use `skills/get` and `resources/read` with a URI such as `skill://pkgsite/overview/SKILL.md`. This tool returns the guide body without YAML frontmatter alongside a structured header. When `header_only` is true, it returns the header as structured content and JSON text.
