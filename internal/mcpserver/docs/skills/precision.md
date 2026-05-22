---
name: pkgsite/precision
description: How to avoid module and package ambiguity.
related: pkgsite/entities, pkgsite/operations
---

# Precision

Package paths can be ambiguous across modules. If a package lookup returns candidates or an ambiguity message, repeat the call with `module_path`.

Use version-pinned calls when answering compatibility questions. Empty `version` means latest and can change.

Do not treat absence of a field as proof unless the raw upstream response makes that absence clear.
