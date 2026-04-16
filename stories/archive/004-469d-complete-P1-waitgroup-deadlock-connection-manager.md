---
id: 004-469d
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.231Z"
---
# WaitGroup pre-incremented with no matching Done() causes ReceiveAll deadlock

## Description
`NewConnectionManager` calls `wg.Add(1)` during construction. `wg.Done()` is never called anywhere in the codebase. `ReceiveAll`'s `cm.wg.Wait()` can never return, causing the receive loop to hang forever after all goroutines finish.

## Acceptance Criteria
- [ ] Remove the phantom `wg.Add(1)` from `NewConnectionManager`
- [ ] `ReceiveAll` returns normally when all connections stop

## Context Files
- `internal/connection/connection_manager.go:28-29` — the spurious `wg.Add(1)`
- `internal/connection/connection_manager.go:102-108` — `ReceiveAll` and the `wg.Wait()`

## Work Log

### 2026-04-16T20:39:38.186Z - Completed by parallel agent - see review file for details

