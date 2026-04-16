---
id: 011-722a
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.662Z"
---
# Dedupe test is broken: empty ids list makes dedup assertion unreachable

## Description
`TestCanInterfaceFilter` in `dedupe_test.go` creates a `DedupeFilterClient` with `ids: []uint32{}`. The first guard clause in `Filter()` returns `false` immediately for any ID not in the list — which is all IDs when the list is empty. The assertion `skip2 == true` (second occurrence suppressed) can never be reached.

## Acceptance Criteria
- [ ] Fix the test to use `[]uint32{42}` (matching the test message ID)
- [ ] Test passes and actually exercises the deduplication code path
- [ ] Add test for: message ID _not_ in watched list is never suppressed

## Context Files
- `internal/client/dedupe/dedupe_test.go:18,67` — the broken test
- `internal/client/dedupe/dedupe.go:32-34` — the guard clause

## Work Log

### 2026-04-16T20:39:38.620Z - Completed by parallel agent - see review file for details

