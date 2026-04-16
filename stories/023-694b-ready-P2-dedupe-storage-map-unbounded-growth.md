---
id: "023-694b"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# Dedupe storage map grows unboundedly — memory leak over time

## Description
`storage map[uint64]time.Time` is never evicted. Entries for messages never seen again accumulate forever. Over days of operation at 100 frames/sec with many distinct message IDs and payloads, this is an unbounded memory leak.

## Acceptance Criteria
- [ ] Periodic eviction of entries older than `timeout` milliseconds
- [ ] Eviction runs on a background ticker (not on every message, to avoid O(n) per-message cost)
- [ ] Memory usage stabilizes during long-running operation

## Context Files
- `internal/client/dedupe/dedupe.go:14-19` — the storage map
- `internal/client/dedupe/dedupe.go:31-57` — Filter() where entries are added
