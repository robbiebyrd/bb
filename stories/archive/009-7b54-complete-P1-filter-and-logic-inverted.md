---
id: 009-7b54
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.111Z"
---
# AND filter logic inverted in FilterClient and CanDataFilter

## Description
Both `FilterClient.Filter` and `CanDataFilter.Filter` use `ArrayContainsFalse()` for the AND operator, which returns true when _any_ filter is false — the opposite of AND semantics. The correct implementation is `ArrayAllTrue()`, as used in `BroadcastClient.testFilterGroup`. `FilterClient` is not wired in production today, but any future use will silently pass traffic that should be blocked.

## Acceptance Criteria
- [ ] `FilterClient.Filter` default branch uses `ArrayAllTrue(filterResults)`
- [ ] `CanDataFilter.Filter` FilterAnd branch uses `ArrayAllTrue(filterValues)`
- [ ] Tests added covering AND-mode behavior for both

## Context Files
- `internal/client/filter/filter.go:29-34` — the inverted AND in FilterClient
- `internal/client/filter/types.go:101-106` — the inverted AND in CanDataFilter
- `internal/client/broadcast/broadcast.go:99-103` — the correct reference implementation

## Work Log

### 2026-04-16T20:39:38.065Z - Completed by parallel agent - see review file for details

