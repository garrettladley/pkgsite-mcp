# Precision

Package paths can be ambiguous across modules. If a package lookup returns candidates or an ambiguity message, repeat the call with `module_path`.

Error responses preserve the upstream `message`, `fixes`, and `candidates` fields. Prefer the suggested containing module from `candidates` or `fixes` instead of guessing which module owns an ambiguous package path.

Use version-pinned calls when answering compatibility questions. Empty `version` means latest and can change.

Do not treat absence of a field as proof unless the raw upstream response makes that absence clear.
