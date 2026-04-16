---
id: "016-759a"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# Discontinue() ignores receiver.Close() error

## Description
`scc.receiver.Close()` returns an error that is completely discarded. `Discontinue()` always returns `nil` regardless, creating a false success signal and a state inconsistency.

## Acceptance Criteria
- [ ] `Discontinue()` returns `fmt.Errorf("close CAN receiver %q: %w", scc.name, err)` on failure

## Context Files
- `internal/connection/socketcan/connection.go:119-124` — `Discontinue()`
