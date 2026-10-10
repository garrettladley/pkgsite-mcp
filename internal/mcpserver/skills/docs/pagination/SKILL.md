---
name: pagination
description: How upstream page tokens and local response truncation work.
metadata:
  related-skills: pkgsite/operations
---

# Pagination

There are two pagination layers. Their continuation fields appear in the JSON `pagination` object.

Upstream pkg.go.dev pagination uses `limit` and `token`. When `upstreamNextPageToken` is nonempty, pass it as `token` to fetch the next upstream page. Keep the rest of the upstream request unchanged, and reset `start_at` to zero. A nonempty upstream token means another page exists even when the current page has no items.

Local display pagination uses `start_at` and `max_tokens`. When `nextStartAt` is non-null, repeat the same tool call with that `start_at` to see the next local batch from the current upstream response. A null `nextStartAt` means the fetched page is fully displayed; it does not imply that upstream pagination is complete.

`fetchedItems` counts the current upstream page; `displayedItems` counts this local batch. `total` preserves the upstream total when available, and otherwise counts the fetched items. Empty successful lists return `items: []`.

The token budget is approximate, using four serialized JSON bytes per token. If even one item cannot fit, the tool reports an error asking for a larger `max_tokens` rather than returning a continuation that cannot advance. Entity lookups return their complete metadata and requested documentation; local pagination applies to list results.

`pkgsite_explain` budgets the combined JSON response. Each successful list lookup has its own pagination under `lookups.<name>.data.pagination`. Use the corresponding individual tool to continue a particular list. When continuing via `pkgsite_explain`, `start_at` applies to every included list.
