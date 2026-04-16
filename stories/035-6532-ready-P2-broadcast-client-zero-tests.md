---
id: "035-6532"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# BroadcastClient has zero tests for its core routing logic

## Description
`BroadcastClient` is the central message routing component — every CAN frame flows through it. None of its behavior is tested: Add/Remove listener management, Broadcast loop, filter application, duplicate-name rejection, testFilterGroup AND/OR semantics.

## Acceptance Criteria
- [ ] Test: `Add` returns error on duplicate name
- [ ] Test: `Remove` returns error on unknown name
- [ ] Test: `Broadcast` routes messages to all registered listeners
- [ ] Test: `Broadcast` skips listeners that don't pass filter
- [ ] Test: `testFilterGroup` correctly applies AND vs OR semantics
- [ ] Test: empty filter list edge case

## Context Files
- `internal/client/broadcast/broadcast.go` — all methods
