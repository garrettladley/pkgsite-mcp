---
name: pkgsite/pagination
description: How upstream page tokens and local response truncation work.
related: pkgsite/operations
---

# Pagination

There are two pagination layers.

Upstream pkg.go.dev pagination uses `limit` and `token`. When a response includes `upstreamNextPageToken`, pass it as `token` to fetch the next upstream page.

Local display pagination uses `start_at` and `max_tokens`. When metadata includes `next_start_at`, repeat the same tool call with that `start_at` to see the next local batch from the current upstream response.
