---
id: "021-680e"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# defer writer.Flush() in CSV NewClient runs when constructor returns, not on close

## Description
`defer writer.Flush()` in `NewClient` runs when `NewClient` returns, not when the CSV client is closed. This is misleading and there is no `Close()` method on `CSVClient`. The header row happens to flush correctly due to this timing, but the pattern is confusing and fragile.

## Acceptance Criteria
- [ ] Remove the `defer`; call `writer.Flush()` explicitly after writing the header
- [ ] Check and handle the Flush error

## Context Files
- `internal/output/csv/client.go:32-44` — constructor with misplaced defer
